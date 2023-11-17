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

	"github.com/bcc-code/bcc-media-platform/backend/asset"
	"github.com/bcc-code/bccm-flows/activities"
	vsactivity "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
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

func VXExportToBMM(ctx workflow.Context, params VXExportChildWorkflowParams) (*VXExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportToBMM")

	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	normalizedFutures := map[string]workflow.Future{}

	langs, err := wfutils.GetMapKeysSafely(ctx, params.MergeResult.AudioFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio file keys: %w", err)
	}

	tempDir, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow temp folder: %w", err)
	}

	// We don't want to upload folders from other workflows that can be triggered at the same export.
	outputFolder := tempDir.Append("bmm")
	err = wfutils.CreateFolder(ctx, outputFolder)
	if err != nil {
		return nil, fmt.Errorf("failed to create output folder: %w", err)
	}

	// Normalize audio
	for _, lang := range langs {
		audio := params.MergeResult.AudioFiles[lang]
		ctx = workflow.WithChildOptions(ctx, wfutils.GetDefaultWorkflowOptions())
		future := wfutils.ExecuteWithQueue(ctx, activities.NormalizeAudioActivity, activities.NormalizeAudioParams{
			FilePath:              audio,
			TargetLUFS:            targetLufs,
			PerformOutputAnalysis: true,
			OutputPath:            tempDir,
		})
		normalizedFutures[lang] = future
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
			f := wfutils.ExecuteWithQueue(ctx, activities.TranscodeToAudioAac, common.AudioInput{
				Path:            audio.FilePath,
				DestinationPath: outputFolder,
				Bitrate:         bitrate,
			})
			encodings = append(encodings, f)
		}

		for _, bitrate := range mp3Bitrates {
			f := wfutils.ExecuteWithQueue(ctx, activities.TranscodeToAudioMP3, common.AudioInput{
				Path:            audio.FilePath,
				DestinationPath: outputFolder,
				Bitrate:         bitrate,
			})
			encodings = append(encodings, f)
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

	// Prepare data for the JSON file
	jsonData := prepareBMMData(audioResults, normalizedResults)
	jsonData.Length = int(params.MergeResult.Duration)
	jsonData.MediabankenID = fmt.Sprintf("%s-%s", params.ParentParams.VXID, HashTitle(params.ExportData.Title))

	jsonData.Title = params.ExportData.Title

	var chapters []asset.Chapter
	err = wfutils.ExecuteWithQueue(ctx, vsactivity.GetChapterDataActivity, vsactivity.GetChapterDataParams{
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
		jsonData.Type = chapter.ChapterType
		if chapter.SongNumber != "" && chapter.SongCollection != "" {
			jsonData.SongCollection = &chapter.SongCollection
			jsonData.SongNumber = &chapter.SongNumber
		}
	}

	marshalled, err := json.Marshal(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	err = wfutils.WriteFile(ctx, outputFolder.Append("bmm.json"), marshalled)
	if err != nil {
		return nil, fmt.Errorf("failed to write JSON file: %w", err)
	}

	ingestFolder := params.ExportData.SafeTitle + "_" + workflow.GetInfo(ctx).OriginalRunID
	err = workflow.ExecuteActivity(ctx, activities.RcloneCopyDir, activities.RcloneCopyDirInput{
		Source:      outputFolder.Rclone(),
		Destination: fmt.Sprintf("bmms3:/int-bmm-mediabanken/" + ingestFolder),
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	// TODO: Trigger as activity?
	trigger := "https://int-bmm-api.brunstad.org/events/mediabanken-export/?path="
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

	return &VXExportResult{
		ID:       params.ParentParams.VXID,
		Title:    params.ExportData.Title,
		Duration: formatSecondsToTimestamp(params.MergeResult.Duration),
	}, nil
}

type BMMData struct {
	MediabankenID    string                    `json:"mediabanken_id"`
	Title            string                    `json:"title"`
	Length           int                       `json:"length"`
	Type             string                    `json:"type"`
	AudioFiles       map[string][]BMMAudioFile `json:"audio_files"`
	PersonsAppearing []string                  `json:"persons_appearing"`
	SongCollection   *string                   `json:"song_collection"`
	SongNumber       *string                   `json:"song_number"`
	RecordedAt       time.Time                 `json:"recorded_at"`
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
				Lufs:            analysis[lang].OutputAnalysis.IntegratedLoudness,
				DynamicRange:    analysis[lang].OutputAnalysis.LoudnessRange,
				Language:        lang,
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
