package workflows

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/activities"
	avidispine "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
	"os"
	"path/filepath"
)

type AssetExportParams struct {
	VXID string
}

type AssetExportResult struct {
}

func AssetExportVX(ctx workflow.Context, params AssetExportParams) (*AssetExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting AssetExport")

	options := GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	var data *vidispine.ExportData

	err := workflow.ExecuteActivity(ctx, avidispine.GetExportDataActivity, avidispine.GetExportDataParams{
		VXID: params.VXID,
	}).Get(ctx, &data)
	if err != nil {
		return nil, err
	}

	logger.Info("Retrieved data from vidispine")

	workflowFolder, err := utils.GetWorkflowOutputFolder(ctx)
	if err != nil {
		return nil, err
	}

	tempFolder := filepath.Join(workflowFolder, "temp")

	err = os.MkdirAll(tempFolder, os.ModePerm)
	if err != nil {
		return nil, err
	}

	downloadablesFolder := filepath.Join(workflowFolder, "downloadables")

	err = os.MkdirAll(downloadablesFolder, os.ModePerm)
	if err != nil {
		return nil, err
	}

	streamsFolder := filepath.Join(workflowFolder, "streams")

	err = os.MkdirAll(streamsFolder, os.ModePerm)
	if err != nil {
		return nil, err
	}

	//defer func() {
	//	_ = os.RemoveAll(tempFolder)
	//}()

	mergeInput := common.MergeInput{
		Title:     data.Title,
		OutputDir: tempFolder,
		WorkDir:   tempFolder,
	}

	var audioMergeInputs = map[string]*common.MergeInput{}
	var subtitleMergeInputs = map[string]*common.MergeInput{}

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

			audioMergeInputs[lan].Items = append(audioMergeInputs[lan].Items, common.MergeInputItem{
				Path:    af.File,
				Start:   clip.InSeconds,
				End:     clip.OutSeconds,
				Streams: af.Channels,
			})
		}

		for lan, sf := range clip.SubtitleFiles {
			if _, ok := subtitleMergeInputs[lan]; !ok {
				subtitleMergeInputs[lan] = &common.MergeInput{
					Title:     data.Title + "-" + lan,
					OutputDir: workflowFolder,
					WorkDir:   tempFolder,
				}
			}

			subtitleMergeInputs[lan].Items = append(subtitleMergeInputs[lan].Items, common.MergeInputItem{
				Path:  sf,
				Start: clip.InSeconds,
				End:   clip.OutSeconds,
			})
		}
	}

	options.TaskQueue = utils.GetTranscodeQueue()
	ctx = workflow.WithActivityOptions(ctx, options)

	var videoFile string
	{
		var result common.MergeResult
		err = workflow.ExecuteActivity(ctx, activities.TranscodeMergeVideo, mergeInput).Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		videoFile = result.Path
	}

	var audioFiles = map[string]string{}
	for lang, mi := range audioMergeInputs {
		var result common.MergeResult
		err = workflow.ExecuteActivity(ctx, activities.TranscodeMergeAudio, *mi).Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		audioFiles[lang] = result.Path
	}

	var subtitleFiles = map[string]string{}
	for lang, mi := range subtitleMergeInputs {
		var result common.MergeResult
		err = workflow.ExecuteActivity(ctx, activities.TranscodeMergeSubtitles, *mi).Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		subtitleFiles[lang] = result.Path
	}

	// ordered by quality
	var videoFiles []string
	{
		inputs := []common.VideoInput{
			{
				Path:            videoFile,
				FrameRate:       25,
				Width:           1920,
				Height:          1080,
				Bitrate:         "5M",
				DestinationPath: tempFolder,
			},
			{
				Path:            videoFile,
				FrameRate:       25,
				Width:           1280,
				Height:          720,
				Bitrate:         "2M",
				DestinationPath: tempFolder,
			},
		}

		for _, input := range inputs {
			var result common.VideoResult
			err = workflow.ExecuteActivity(ctx, activities.TranscodeToVideoH264, input).Get(ctx, &result)
			if err != nil {
				return nil, err
			}
			videoFiles = append(videoFiles, result.OutputPath)
		}
	}

	var compressedAudioFiles = map[string]string{}
	for lang, path := range audioFiles {
		var result common.AudioResult
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToAudioAac, common.AudioInput{
			Path:            path,
			Bitrate:         "128k",
			DestinationPath: tempFolder,
		}).Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		compressedAudioFiles[lang] = result.OutputPath
	}

	for lang, path := range compressedAudioFiles {
		for _, f := range videoFiles {
			base := filepath.Base(f)
			err = workflow.ExecuteActivity(ctx, activities.TranscodeMux, common.MuxInput{
				FileName:        base[:len(base)-len(filepath.Ext(base))] + "-" + lang,
				DestinationPath: downloadablesFolder,
				VideoFilePath:   f,
				AudioFilePaths:  map[string]string{lang: path},
			}).Get(ctx, nil)
			if err != nil {
				return nil, err
			}
		}
	}

	var languageKeys []string

	for l := range compressedAudioFiles {
		languageKeys = append(languageKeys, l)
	}

	languages := utils.LanguageKeysToOrderedLanguages(languageKeys)

	for index, chunk := range lo.Chunk(languages, 16) {
		var audioFilePaths = map[string]string{}
		for _, lang := range chunk {
			audioFilePaths[lang.ISO6391] = compressedAudioFiles[lang.ISO6391]
		}

		for _, f := range videoFiles {
			base := filepath.Base(f)

			err = workflow.ExecuteActivity(ctx, activities.TranscodeMux, common.MuxInput{
				FileName:        base[:len(base)-len(filepath.Ext(base))] + fmt.Sprintf("-%d", index),
				DestinationPath: streamsFolder,
				VideoFilePath:   f,
				AudioFilePaths:  audioFilePaths,
			}).Get(ctx, nil)
		}
	}

	return nil, nil
}
