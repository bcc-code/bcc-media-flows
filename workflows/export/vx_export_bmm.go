package export

import (
	"crypto/sha1"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/languages"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/telegram"

	pcommon "github.com/bcc-code/bcc-media-platform/backend/common"

	platform_activities "github.com/bcc-code/bcc-media-flows/activities/platform"
	"github.com/bcc-code/bcc-media-flows/services/rclone"

	"github.com/bcc-code/bcc-media-flows/activities"
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

// broken languages will be skipped during export
var brokenTranscription = map[string]struct{}{
	"kha": {},
	"mal": {},
}

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

	wfutils.SendTelegramText(ctx, telegram.ChatBMM, fmt.Sprintf("🟦 Exporting to BMM - `%s`", params.ExportData.Title))

	langs, err := wfutils.GetMapKeysSafely(ctx, params.MergeResult.AudioFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio file keys: %w", err)
	}

	// We don't want to upload folders from other workflows that can be triggered at the same export.
	err = wfutils.CreateFolder(ctx, params.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create output folder: %w", err)
	}

	normalizedResults, err := normalizeAudioPerLanguage(ctx, params, langs)
	if err != nil {
		return nil, err
	}

	audioResults, err := encodeAudioPerLanguage(ctx, params, langs, normalizedResults)
	if err != nil {
		return nil, err
	}

	err = moveTranscriptsToOutput(ctx, params)
	if err != nil {
		return nil, err
	}

	var chapters []asset.TimedMetadata
	err = wfutils.Execute(ctx, activities.Platform.GetTimedMetadataChaptersActivity, platform_activities.GetTimedMetadataChaptersParams{
		Clips: params.ExportData.Clips,
	}).Get(ctx, &chapters)
	if err != nil {
		return nil, err
	}

	jsonData, err := makeBMMJSON(ctx, params, audioResults, normalizedResults, chapters)
	if err != nil {
		return nil, err
	}

	err = wfutils.WriteFile(ctx, params.OutputDir.Append("bmm.json"), jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to write JSON file: %w", err)
	}

	config := getBMMDestinationConfig(params.ExportDestination)

	ingestFolder := params.ExportData.SafeTitle + "_" + workflow.GetInfo(ctx).OriginalRunID
	err = wfutils.RcloneCopyDir(ctx, params.OutputDir.Rclone(), config.Bucket+ingestFolder, rclone.PriorityNormal)
	if err != nil {
		return nil, err
	}

	_, err = wfutils.Execute(ctx, activities.Util.TriggerBMMImport, activities.TriggerBMMImportInput{
		BaseURL:      config.BaseURL,
		IngestFolder: ingestFolder,
	}).Result(ctx)
	if err != nil {
		return nil, err
	}

	// The emoji here is blue because BMM produces messages in the same Telegram channel and we want
	// only the last one to be green.
	notifyExportDone(ctx, telegram.ChatBMM, params, params.ExportDestination.Value, '🟦')

	return &VXExportResult{
		ID:       params.ParentParams.VXID,
		Title:    params.ExportData.Title,
		Duration: formatSecondsToTimestamp(params.MergeResult.Duration),
	}, nil
}

// normalizeAudioPerLanguage schedules every language before waiting on any of them, so
// they normalize in parallel. It rewrites params.MergeResult.AudioFiles to the
// normalized copies, which the map being shared makes visible to the caller.
func normalizeAudioPerLanguage(ctx workflow.Context, params VXExportChildWorkflowParams, langs []string) (map[string]activities.NormalizeAudioResult, error) {
	logger := workflow.GetLogger(ctx)

	futures := map[string]workflow.Future{}
	for _, lang := range langs {
		futures[lang] = wfutils.Execute(ctx, activities.Audio.NormalizeAudioActivity, activities.NormalizeAudioParams{
			FilePath:              params.MergeResult.AudioFiles[lang],
			TargetLUFS:            targetLufs,
			PerformOutputAnalysis: true,
			OutputPath:            params.TempDir,
		}).Future
	}

	results := map[string]activities.NormalizeAudioResult{}
	for _, lang := range langs {
		result := activities.NormalizeAudioResult{}
		err := futures[lang].Get(ctx, &result)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize audio for language %s: %w", lang, err)
		}

		logger.Debug("Normalized audio for language", lang, result)
		results[lang] = result
		params.MergeResult.AudioFiles[lang] = result.FilePath
	}

	return results, nil
}

func encodeAudioPerLanguage(
	ctx workflow.Context,
	params VXExportChildWorkflowParams,
	langs []string,
	normalized map[string]activities.NormalizeAudioResult,
) (map[string][]common.AudioResult, error) {
	futures := map[string][]workflow.Future{}
	for _, lang := range langs {
		audio := normalized[lang]

		var encodings []workflow.Future
		for _, bitrate := range aacBitrates {
			encodings = append(encodings, wfutils.Execute(ctx, activities.Audio.TranscodeToAudioAac, common.AudioInput{
				Path:            audio.FilePath,
				DestinationPath: params.OutputDir,
				Bitrate:         bitrate,
			}).Future)
		}

		for _, bitrate := range mp3Bitrates {
			encodings = append(encodings, wfutils.Execute(ctx, activities.Audio.TranscodeToAudioMP3, common.AudioInput{
				Path:            audio.FilePath,
				DestinationPath: params.OutputDir,
				Bitrate:         bitrate,
				ForceCBR:        true,
			}).Future)
		}

		futures[lang] = encodings
	}

	results := map[string][]common.AudioResult{}
	for _, lang := range langs {
		var encodings []common.AudioResult
		for _, future := range futures[lang] {
			var res common.AudioResult
			err := future.Get(ctx, &res)
			if err != nil {
				return nil, fmt.Errorf("failed to transcode audio for language %s: %w", lang, err)
			}

			encodings = append(encodings, res)
		}

		results[lang] = encodings
	}

	return results, nil
}

func moveTranscriptsToOutput(ctx workflow.Context, params VXExportChildWorkflowParams) error {
	langs, err := wfutils.GetMapKeysSafely(ctx, params.MergeResult.JSONTranscript)
	if err != nil {
		return err
	}

	for _, lang := range langs {
		transcript := params.MergeResult.JSONTranscript[lang]

		err = wfutils.MoveFile(ctx, transcript, params.OutputDir.Append(transcript.Base()), rclone.PriorityNormal)
		if err != nil {
			return err
		}
	}

	return nil
}

func makeBMMJSON(
	ctx workflow.Context,
	params VXExportChildWorkflowParams,
	audioResults map[string][]common.AudioResult,
	normalizedResults map[string]activities.NormalizeAudioResult,
	chapters []asset.TimedMetadata,
) ([]byte, error) {
	logger := workflow.GetLogger(ctx)

	// Prepare data for the JSON file
	jsonData := prepareBMMData(ctx, audioResults, normalizedResults)
	jsonData.Length = int(params.MergeResult.Duration)
	jsonData.MediabankenID = fmt.Sprintf("%s-%s", params.ParentParams.VXID, HashTitle(params.ExportData.Title))
	jsonData.ImportDate = params.ExportData.ImportDate
	jsonData.ForceReplaceTranscription = params.ForceReplaceTranscription

	if params.ExportData.BmmTitle != nil && *params.ExportData.BmmTitle != "" {
		jsonData.Title = *params.ExportData.BmmTitle
	}
	jsonData.TrackID = params.ExportData.BmmTrackID

	jsonData.TranscriptionFiles = bmmTranscriptionFiles(ctx, params.MergeResult.JSONTranscript)

	if len(chapters) > 0 {
		recordedBase := workflow.Now(ctx).Truncate(time.Hour * 6)
		if params.ExportData.ImportDate != nil {
			recordedBase = *params.ExportData.ImportDate
		}

		applyChapterToBMMData(&jsonData, chapters[0], recordedBase)
	}

	if len(jsonData.PersonsAppearing) == 0 && jsonData.SongNumber == nil && jsonData.Title == "" {
		logger.Info("No BMM data found, using default title", "title", params.ExportData.Title)
		jsonData.Title = params.ExportData.Title
	}

	return wfutils.MarshalJson(ctx, jsonData)
}

// bmmTranscriptionFiles keys the transcripts by the same language codes as the audio,
// which BMM needs, and drops the ones known to be broken.
//
// Nothing here fails the export. A missing transcription costs a transcription and
// nothing else, and an episode without one is still an episode people can listen to.
func bmmTranscriptionFiles(ctx workflow.Context, transcripts map[string]paths.Path) map[string]string {
	langs, _ := wfutils.GetMapKeysSafely(ctx, transcripts)

	files := map[string]string{}
	for _, lang := range langs {
		bmmLang := lang
		if val, ok := languages.LanguagesByISOTwoLetter[lang]; ok {
			bmmLang = val.ISO6391
		}

		if _, skip := brokenTranscription[bmmLang]; skip {
			continue
		}

		files[bmmLang] = transcripts[lang].Base()
	}

	return files
}

func applyChapterToBMMData(data *BMMData, chapter asset.TimedMetadata, recordedBase time.Time) {
	for _, p := range chapter.Persons {
		if !lo.Contains(data.PersonsAppearing, p) {
			data.PersonsAppearing = append(data.PersonsAppearing, p)
		}
	}

	recordedAt := recordedBase.Add(time.Duration(chapter.Timestamp * float64(time.Second)))
	data.RecordedAt = &recordedAt

	data.StartsAt = chapter.Timestamp
	data.Type = chapter.ContentType

	if chapter.SongNumber != "" && chapter.SongCollection != "" {
		data.SongCollection = &chapter.SongCollection
		data.SongNumber = &chapter.SongNumber
	}

	if chapter.ContentType == pcommon.ContentTypeSong.Value && chapter.SongCollection == "" {
		data.Title = chapter.Title
	}

	if len(data.PersonsAppearing) == 0 && data.SongNumber == nil && data.Title == "" {
		data.Title = chapter.Title
	}
}

type BMMData struct {
	MediabankenID             string                    `json:"mediabanken_id"`
	StartsAt                  float64                   `json:"starts_at"`
	Title                     string                    `json:"title"`
	Length                    int                       `json:"length"`
	Type                      string                    `json:"type"`
	TrackID                   *int                      `json:"track_id"`
	AudioFiles                map[string][]BMMAudioFile `json:"audio_files"`
	TranscriptionFiles        map[string]string         `json:"transcription_files"`
	PersonsAppearing          []string                  `json:"persons_appearing"`
	SongCollection            *string                   `json:"song_collection"`
	SongNumber                *string                   `json:"song_number"`
	RecordedAt                *time.Time                `json:"recorded_at"`
	ImportDate                *time.Time                `json:"import_date"`
	ForceReplaceTranscription bool                      `json:"force_replace_transcription"`
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

func prepareBMMData(ctx workflow.Context, audioFiles map[string][]common.AudioResult, analysis map[string]activities.NormalizeAudioResult) BMMData {
	out := BMMData{
		AudioFiles: map[string][]BMMAudioFile{},
	}

	audioFileKeys, err := wfutils.GetMapKeysSafely(ctx, audioFiles)
	if err != nil {
		return out
	}

	for _, lang := range audioFileKeys {
		variations := audioFiles[lang]
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
