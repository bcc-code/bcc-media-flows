package export

import (
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"github.com/bcc-code/bccm-flows/workflows"
	"go.temporal.io/sdk/workflow"
)

type MergeExportDataResult struct {
	Duration      float64
	VideoFile     string
	AudioFiles    map[string]string
	SubtitleFiles map[string]string
}

type MergeExportDataParams struct {
	ExportData *vidispine.ExportData
	OutputPath string
	TempPath   string
}

func MergeExportData(ctx workflow.Context, params MergeExportDataParams) (*MergeExportDataResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting MergeExportData")
	data := params.ExportData

	mergeInput, audioMergeInputs, subtitleMergeInputs := exportDataToMergeInputs(data, params.TempPath, params.OutputPath)

	options := workflows.GetDefaultActivityOptions()
	options.TaskQueue = utils.GetTranscodeQueue()
	ctx = workflow.WithActivityOptions(ctx, options)
	videoTask := workflow.ExecuteActivity(ctx, activities.TranscodeMergeVideo, mergeInput)

	var audioTasks = map[string]workflow.Future{}
	{
		keys, err := wfutils.GetMapKeysSafely(ctx, audioMergeInputs)
		if err != nil {
			return nil, err
		}
		for _, lang := range keys {
			mi := audioMergeInputs[lang]
			audioTasks[lang] = workflow.ExecuteActivity(ctx, activities.TranscodeMergeAudio, *mi)
		}
	}

	var subtitleTasks = map[string]workflow.Future{}
	{
		keys, err := wfutils.GetMapKeysSafely(ctx, subtitleMergeInputs)
		if err != nil {
			return nil, err
		}
		for _, lang := range keys {
			mi := subtitleMergeInputs[lang]
			subtitleTasks[lang] = workflow.ExecuteActivity(ctx, activities.TranscodeMergeSubtitles, *mi)
		}

	}
	var videoFile string
	{
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

func exportDataToMergeInputs(data *vidispine.ExportData, tempFolder, subtitlesFolder string) (
	mergeInput common.MergeInput,
	audioMergeInputs map[string]*common.MergeInput,
	subtitleMergeInputs map[string]*common.MergeInput,
) {
	mergeInput = common.MergeInput{
		Title:     data.Title,
		OutputDir: tempFolder,
		WorkDir:   tempFolder,
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
					OutputDir: tempFolder,
					WorkDir:   tempFolder,
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
					OutputDir: subtitlesFolder,
					WorkDir:   tempFolder,
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