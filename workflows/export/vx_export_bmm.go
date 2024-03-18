package export

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/common"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/bcc-code/bcc-media-platform/backend/asset"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
)

// https://support.spotify.com/us/artists/article/audio-file-formats/
var aacBitrates = []string{"128k", "256k"}

// This is what seems to be used today
var mp3Bitrates = []string{"256k"}

// Target LUFS for all audio files going to BMM
// This is based on what Spotify uses
const targetLufs = -14.0

type bmmConfig struct {
	Bucket  string
	BaseURL string
}

func getBMMDestinationConfig(dst AssetExportDestination) bmmConfig {
	if dst == AssetExportDestinationBMM {
		return bmmConfig{
			Bucket:  "bmms3:/prod-bmm-mediabanken/",
			BaseURL: "https://bmm-api.brunstad.org",
		}
	} else if dst == AssetExportDestinationBMMIntegration {
		return bmmConfig{
			Bucket:  "bmms3:/int-bmm-mediabanken/",
			BaseURL: "https://int-bmm-api.brunstad.org",
		}
	}

	panic(fmt.Errorf("unsupported destination: %s", dst))
}

// VXExportToBMM exports the specified vx params to BMM
// It normalizes the audio, encodes it to AAC and MP3, and uploads it to BMM
func VXExportToBMM(ctx workflow.Context, params VXExportChildWorkflowParams) (*VXExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportToBMM")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	normalizedFutures := map[string]workflow.Future{}

	langs, err := wfutils.GetMapKeysSafely(ctx, params.MergeResult.AudioFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio file keys: %w", err)
	}

	// We don't want to upload folders from other workflows that can be triggered at the same export.
	err = wfutils.CreateFolder(ctx, params.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create output folder: %w", err)
	}

	// Normalize audio
	for _, lang := range langs {
		audio := params.MergeResult.AudioFiles[lang]
		future := wfutils.Execute(ctx, activities.Audio.NormalizeAudioActivity, activities.NormalizeAudioParams{
			FilePath:              audio,
			TargetLUFS:            targetLufs,
			PerformOutputAnalysis: true,
			OutputPath:            params.TempDir,
		})
		normalizedFutures[lang] = future.Future
	}

	normalizedResults := map[string]activities.NormalizeAudioResult{}
	for _, lang := range langs {
		future := normalizedFutures[lang]
		normalizedRes := activities.NormalizeAudioResult{}
		err := future.Get(ctx, &normalizedRes)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize audio for language %s: %w", lang, err)
		}

		logger.Debug("Normalized audio for language %s: %v", lang, normalizedRes)
		normalizedResults[lang] = normalizedRes
		params.MergeResult.AudioFiles[lang] = normalizedRes.FilePath
	}

	// Encode to AAC and MP3
	encodingFutures := map[string][]workflow.Future{}
	for _, lang := range langs {
		audio := normalizedResults[lang]
		var encodings []workflow.Future
		for _, bitrate := range aacBitrates {
			f := wfutils.Execute(ctx, activities.Audio.TranscodeToAudioAac, common.AudioInput{
				Path:            audio.FilePath,
				DestinationPath: params.OutputDir,
				Bitrate:         bitrate,
			})
			encodings = append(encodings, f.Future)
		}

		for _, bitrate := range mp3Bitrates {
			f := wfutils.Execute(ctx, activities.Audio.TranscodeToAudioMP3, common.AudioInput{
				Path:            audio.FilePath,
				DestinationPath: params.OutputDir,
				Bitrate:         bitrate,
			})
			encodings = append(encodings, f.Future)
		}

		encodingFutures[lang] = encodings
	}

	audioResults := map[string][]common.AudioResult{}
	for _, lang := range langs {
		futures := encodingFutures[lang]
		var encodings []common.AudioResult
		for _, future := range futures {
			var res common.AudioResult
			err := future.Get(ctx, &res)
			if err != nil {
				return nil, fmt.Errorf("failed to transcode audio for language %s: %w", lang, err)
			}
			encodings = append(encodings, res)
		}

		audioResults[lang] = encodings
	}

	{
		// Move the transcription files to the output folder
		keys, err := wfutils.GetMapKeysSafely(ctx, params.MergeResult.JSONTranscript)
		if err != nil {
			return nil, err
		}
		for _, lang := range keys {
			p := params.MergeResult.JSONTranscript[lang]

			err = wfutils.MoveFile(ctx, p, params.OutputDir.Append(p.Base()))
			if err != nil {
				return nil, err
			}
		}
	}

	// Prepare data for the JSON file
	jsonData := prepareBMMData(audioResults, normalizedResults)
	jsonData.Length = int(params.MergeResult.Duration)
	jsonData.MediabankenID = fmt.Sprintf("%s-%s", params.ParentParams.VXID, HashTitle(params.ExportData.Title))
	jsonData.ImportDate = params.ExportData.ImportDate
	jsonData.TranscriptionFiles = map[string]string{}
	jsonData.Title = params.ExportData.Title
	if params.ExportData.BmmTitle != nil {
		jsonData.Title = *params.ExportData.BmmTitle
	}
	jsonData.TrackID = params.ExportData.BmmTrackID

	for lang, transcript := range params.MergeResult.JSONTranscript {
		jsonData.TranscriptionFiles[lang] = transcript.Base()
	}

	var chapters []asset.Chapter
	err = wfutils.Execute(ctx, vsactivity.GetChapterDataActivity, vsactivity.GetChapterDataParams{
		ExportData: &params.ExportData,
	}).Get(ctx, &chapters)
	if err != nil {
		return nil, err
	}

	if len(chapters) > 0 {
		chapter := chapters[0]
		for _, p := range chapter.Persons {
			if !lo.Contains(jsonData.PersonsAppearing, p) {
				jsonData.PersonsAppearing = append(jsonData.PersonsAppearing, p)
			}
		}

		d := time.Now().Truncate(time.Hour * 6)
		if params.ExportData.ImportDate != nil {
			d = *params.ExportData.ImportDate
		}
		chaperRecordedAt := d.Add(time.Duration(chapter.Timestamp * float64(time.Second)))
		jsonData.RecordedAt = &chaperRecordedAt

		jsonData.StartsAt = chapter.Timestamp
		jsonData.Type = chapter.ChapterType

		if jsonData.Type == "sing_along" {
			jsonData.Type = "singsong"
		}
		if chapter.SongNumber != "" && chapter.SongCollection != "" {
			jsonData.SongCollection = &chapter.SongCollection
			jsonData.SongNumber = &chapter.SongNumber
		}
	}

	marshalled, err := json.Marshal(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	err = wfutils.WriteFile(ctx, params.OutputDir.Append("bmm.json"), marshalled)
	if err != nil {
		return nil, fmt.Errorf("failed to write JSON file: %w", err)
	}

	config := getBMMDestinationConfig(params.ExportDestination)

	ingestFolder := params.ExportData.SafeTitle + "_" + workflow.GetInfo(ctx).OriginalRunID
	err = wfutils.Execute(ctx, activities.RcloneCopyDir, activities.RcloneCopyDirInput{
		Source:      params.OutputDir.Rclone(),
		Destination: fmt.Sprintf(config.Bucket + ingestFolder),
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	trigger := config.BaseURL + "/events/mediabanken-export/?path="
	jsonS3Path := path.Join(ingestFolder, "bmm.json")
	trigger += url.QueryEscape(jsonS3Path)

	resp, err := http.Post(trigger, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to trigger BMM: %w", err)
	}

	resp.Body.Close()

	if resp.StatusCode > 200 {
		return nil, fmt.Errorf("failed to trigger BMM: %s", resp.Status)
	}

	notifyExportDone(ctx, params, "bmm")

	return &VXExportResult{
		ID:       params.ParentParams.VXID,
		Title:    params.ExportData.Title,
		Duration: formatSecondsToTimestamp(params.MergeResult.Duration),
	}, nil
}

type BMMData struct {
	MediabankenID      string                    `json:"mediabanken_id"`
	StartsAt           float64                   `json:"starts_at"`
	Title              string                    `json:"title"`
	Length             int                       `json:"length"`
	Type               string                    `json:"type"`
	TrackID            *int                      `json:"track_id"`
	AudioFiles         map[string][]BMMAudioFile `json:"audio_files"`
	TranscriptionFiles map[string]string         `json:"transcription_files"`
	PersonsAppearing   []string                  `json:"persons_appearing"`
	SongCollection     *string                   `json:"song_collection"`
	SongNumber         *string                   `json:"song_number"`
	RecordedAt         *time.Time                `json:"recorded_at"`
	ImportDate         *time.Time                `json:"import_date"`
}

type BMMAudioFile struct {
	Bitrate         int64   `json:"bitrate"`
	VariableBitrate bool    `json:"variable_bitrate"`
	ChannelCount    int     `json:"channel_count"`
	Path            string  `json:"path"`
	Lufs            float64 `json:"lufs"`
	DynamicRange    float64 `json:"dynamic_range"`
	Peak            float64 `json:"peak"`
	Language        string  `json:"language"`
	MimeType        string  `json:"mime_type"`
	Size            int64   `json:"size"`
}

func prepareBMMData(audioFiles map[string][]common.AudioResult, analysis map[string]activities.NormalizeAudioResult) BMMData {
	out := BMMData{
		AudioFiles: map[string][]BMMAudioFile{},
	}

	for lang, variations := range audioFiles {
		var langFiles []BMMAudioFile

		for _, file := range variations {

			// BMM needs an integer bitrate
			bitrate, _ := strconv.ParseInt(strings.ReplaceAll(file.Bitrate, "k", ""), 10, 64)
			bitrate *= 1000

			f := BMMAudioFile{
				Bitrate:         bitrate,
				VariableBitrate: true,
				ChannelCount:    2,
				Path:            path.Base(file.OutputPath.Local()), // This needs to be relative to the resultintg JSON file
				Language:        lang,
				Size:            file.FileSize,
			}

			outputAnalysis := analysis[lang].OutputAnalysis
			if outputAnalysis != nil {
				f.Lufs = outputAnalysis.IntegratedLoudness
				f.DynamicRange = outputAnalysis.LoudnessRange
			}

			switch {
			case file.Format == "aac":
				f.MimeType = "audio/aac"
			case file.Format == "mp3":
				f.MimeType = "audio/mpeg"
			default:
				// Since this should never happen (only during dev), we panic
				panic(fmt.Errorf("unsupported audio format: %s", file.Format))
			}

			langFiles = append(langFiles, f)
		}

		out.AudioFiles[lang] = langFiles
	}

	return out

}

func HashTitle(title string) string {
	hash := sha1.Sum([]byte(title))
	return fmt.Sprintf("%x", hash)[0:8]
}
