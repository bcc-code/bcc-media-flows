package export

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"github.com/bcc-code/bccm-flows/workflows"
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

	// Normalize audio
	for _, lang := range langs {
		audio := params.MergeResult.AudioFiles[lang]
		ctx = workflow.WithChildOptions(ctx, wfutils.GetDefaultWorkflowOptions())
		future := workflow.ExecuteChildWorkflow(ctx, workflows.NormalizeAudioLevelWorkflow, workflows.NormalizeAudioParams{
			FilePath:              audio,
			TargetLUFS:            targetLufs,
			PerformOutputAnalysis: true,
		})
		normalizedFutures[lang] = future
	}

	normalizedResults := map[string]workflows.NormalizeAudioResult{}
	for _, lang := range langs {
		future := normalizedFutures[lang]
		normalizedRes := workflows.NormalizeAudioResult{}
		err := future.Get(ctx, &normalizedRes)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize audio for language %s: %w", lang, err)
		}

		logger.Debug("Normalized audio for language %s: %v", lang, normalizedRes)
		normalizedResults[lang] = normalizedRes
		params.MergeResult.AudioFiles[lang] = normalizedRes.FilePath
	}

	outputFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow temp folder: %w", err)
	}

	// Encode to AAC and MP3

	encodingFutures := map[string][]workflow.Future{}
	for _, lang := range langs {
		audio := normalizedResults[lang]
		encodings := []workflow.Future{}
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
		encodings := []common.AudioResult{}
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
	marshalled, err := json.Marshal(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	err = wfutils.WriteFile(ctx, path.Join(outputFolder, "bmm.json"), marshalled)
	if err != nil {
		return nil, fmt.Errorf("failed to write JSON file: %w", err)
	}

	ingestFolder := params.ExportData.Title + "_" + workflow.GetInfo(ctx).OriginalRunID
	err = workflow.ExecuteActivity(ctx, activities.RcloneCopyDir, activities.RcloneCopyDirInput{
		Source:      strings.Replace(outputFolder, utils.GetIsilonPrefix()+"/", "isilon:isilon/", 1),
		Destination: fmt.Sprintf("bmms3:/int-bmm-mediabanken/" + ingestFolder),
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	// TODO: Trigger BMM import

	return &VXExportResult{
		ID:       params.ParentParams.VXID,
		Title:    params.ExportData.Title,
		Duration: formatSecondsToTimestamp(params.MergeResult.Duration),
	}, nil
}

type BMMData struct {
	MediabankenID string `json:"mediabanken_id"`
	Title         string `json:"title"`
	Length        int    `json:"length"`
	Type          string `json:"type"`
	AudioFiles    map[string][]BMMAudioFile
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

func prepareBMMData(audioFiles map[string][]common.AudioResult, analysis map[string]workflows.NormalizeAudioResult) BMMData {
	out := BMMData{
		AudioFiles: map[string][]BMMAudioFile{},
	}

	for lang, variations := range audioFiles {

		langFiles := []BMMAudioFile{}

		for _, file := range variations {

			// BMM needs an integer bitrate
			bitrate, _ := strconv.ParseInt(strings.ReplaceAll(file.Bitrate, "k", ""), 10, 64)
			bitrate *= 1000

			f := BMMAudioFile{
				Bitrate:         bitrate,
				VariableBitrate: true,
				ChannelCount:    2,
				Path:            path.Base(file.OutputPath), // This needs to be relative to the resultintg JSON file
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
