package export

import (
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"go.temporal.io/sdk/workflow"
)

type MergeExportDataResult struct {
	Duration      float64
	VideoFile     string
	AudioFiles    map[string]string
	SubtitleFiles map[string]string
}

type MergeExportDataParams struct {
	ExportData    *vidispine.ExportData
	SubtitlesDir  string
	TempDir       string
	MakeVideo     bool
	MakeSubtitles bool
	MakeAudio     bool
}

func MergeExportData(ctx workflow.Context, params MergeExportDataParams) (*MergeExportDataResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting MergeExportData")
	data := params.ExportData

	mergeInput, audioMergeInputs, subtitleMergeInputs := exportDataToMergeInputs(data, params.TempDir, params.SubtitlesDir)

	options := wfutils.GetDefaultActivityOptions()
	options.TaskQueue = utils.GetTranscodeQueue()
	ctx = workflow.WithActivityOptions(ctx, options)

	var audioTasks = map[string]workflow.Future{}
	if params.MakeAudio {
		keys, err := wfutils.GetMapKeysSafely(ctx, audioMergeInputs)
		if err != nil {
			return nil, err
		}
		for _, lang := range keys {
			mi := audioMergeInputs[lang]
			audioTasks[lang] = wfutils.ExecuteWithQueue(ctx, activities.TranscodeMergeAudio, *mi)
		}
	}

	var subtitleTasks = map[string]workflow.Future{}
	if params.MakeSubtitles {
		keys, err := wfutils.GetMapKeysSafely(ctx, subtitleMergeInputs)
		if err != nil {
			return nil, err
		}
		for _, lang := range keys {
			mi := subtitleMergeInputs[lang]
			subtitleTasks[lang] = wfutils.ExecuteWithQueue(ctx, activities.TranscodeMergeSubtitles, *mi)
		}

	}

	var videoFile string
	if params.MakeVideo {
		videoTask := wfutils.ExecuteWithQueue(ctx, activities.TranscodeMergeVideo, mergeInput)
		var result common.MergeResult
		err := videoTask.Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		videoFile = result.Path
	}

	var audioFiles = map[string]string{}
	{
		keys, err := wfutils.GetMapKeysSafely(ctx, audioTasks)
		if err != nil {
			return nil, err
		}
		for _, lang := range keys {
			task := audioTasks[lang]
			var result common.MergeResult
			err = task.Get(ctx, &result)
			if err != nil {
				return nil, err
			}
			audioFiles[lang] = result.Path
		}
	}

	var subtitleFiles = map[string]string{}
	{
		keys, err := wfutils.GetMapKeysSafely(ctx, subtitleTasks)
		if err != nil {
			return nil, err
		}
		for _, lang := range keys {
			task := subtitleTasks[lang]
			var result common.MergeResult
			err = task.Get(ctx, &result)
			if err != nil {
				return nil, err
			}
			subtitleFiles[lang] = result.Path
		}
	}

	return &MergeExportDataResult{
		Duration:      mergeInput.Duration,
		VideoFile:     videoFile,
		AudioFiles:    audioFiles,
		SubtitleFiles: subtitleFiles,
	}, nil
}

func exportDataToMergeInputs(data *vidispine.ExportData, tempDir, subtitlesDir string) (
	mergeInput common.MergeInput,
	audioMergeInputs map[string]*common.MergeInput,
	subtitleMergeInputs map[string]*common.MergeInput,
) {
	mergeInput = common.MergeInput{
		Title:     data.Title,
		OutputDir: tempDir,
		WorkDir:   tempDir,
	}

	audioMergeInputs = map[string]*common.MergeInput{}
	subtitleMergeInputs = map[string]*common.MergeInput{}

	for _, clip := range data.Clips {
		mergeInput.Duration += clip.OutSeconds - clip.InSeconds
		mergeInput.Items = append(mergeInput.Items, common.MergeInputItem{
			Path:  clip.VideoFile,
			Start: clip.InSeconds,
			End:   clip.OutSeconds,
		})

		for lan, af := range clip.AudioFiles {
			if _, ok := audioMergeInputs[lan]; !ok {
				audioMergeInputs[lan] = &common.MergeInput{
					Title:     data.Title + "-" + lan,
					OutputDir: tempDir,
					WorkDir:   tempDir,
				}
			}

			audioMergeInputs[lan].Duration += clip.OutSeconds - clip.InSeconds
			audioMergeInputs[lan].Items = append(audioMergeInputs[lan].Items, common.MergeInputItem{
				Path:    af.File,
				Start:   clip.InSeconds,
				End:     clip.OutSeconds,
				Streams: af.Streams,
			})
		}

		for lan, sf := range clip.SubtitleFiles {
			if _, ok := subtitleMergeInputs[lan]; !ok {
				subtitleMergeInputs[lan] = &common.MergeInput{
					Title:     data.Title + "-" + lan,
					OutputDir: subtitlesDir,
					WorkDir:   tempDir,
				}
			}

			subtitleMergeInputs[lan].Duration += clip.OutSeconds - clip.InSeconds
			subtitleMergeInputs[lan].Items = append(subtitleMergeInputs[lan].Items, common.MergeInputItem{
				Path:  sf,
				Start: clip.InSeconds,
				End:   clip.OutSeconds,
			})
		}
	}

	return
}
