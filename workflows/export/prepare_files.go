package export

import (
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"go.temporal.io/sdk/workflow"
)

type PrepareFilesParams struct {
	OutputPath    string
	VideoFile     string
	WatermarkPath string
	AudioFiles    map[string]string
}

type PrepareFilesResult struct {
	VideoFiles map[string]string
	AudioFiles map[string]string
}

func PrepareFiles(ctx workflow.Context, params PrepareFilesParams) (*PrepareFilesResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting PrepareFiles")

	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	ctx = workflow.WithTaskQueue(ctx, utils.GetTranscodeQueue())
	tempFolder := params.OutputPath

	var videoTasks = map[string]workflow.Future{}
	{
		videoFile := params.VideoFile
		qualities := map[string]common.VideoInput{
			r1080p: {
				Path:            videoFile,
				Width:           1920,
				Height:          1080,
				Bitrate:         "8M",
				BufferSize:      "5M",
				DestinationPath: tempFolder,
				WatermarkPath:   params.WatermarkPath,
			},
			r720p: {
				Path:            videoFile,
				Width:           1280,
				Height:          720,
				Bitrate:         "3M",
				DestinationPath: tempFolder,
				WatermarkPath:   params.WatermarkPath,
			},
			r540p: {
				Path:            videoFile,
				Width:           960,
				Height:          540,
				Bitrate:         "1900k",
				DestinationPath: tempFolder,
				WatermarkPath:   params.WatermarkPath,
			},
			r360p: {
				Path:            videoFile,
				Width:           640,
				Height:          360,
				Bitrate:         "980k",
				DestinationPath: tempFolder,
				WatermarkPath:   params.WatermarkPath,
			},
			r270p: {
				Path:            videoFile,
				Width:           480,
				Height:          270,
				Bitrate:         "610k",
				DestinationPath: tempFolder,
				WatermarkPath:   params.WatermarkPath,
			},
			r180p: {
				Path:            videoFile,
				Width:           320,
				Height:          180,
				Bitrate:         "320k",
				DestinationPath: tempFolder,
				WatermarkPath:   params.WatermarkPath,
			},
		}

		keys, err := wfutils.GetMapKeysSafely(ctx, qualities)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			input := qualities[key]
			videoTasks[key] = workflow.ExecuteActivity(ctx, activities.TranscodeToVideoH264, input)
		}
	}

	var audioTasks = map[string]workflow.Future{}
	{
		keys, err := wfutils.GetMapKeysSafely(ctx, params.AudioFiles)
		if err != nil {
			return nil, err
		}
		for _, lang := range keys {
			path := params.AudioFiles[lang]
			audioTasks[lang] = workflow.ExecuteActivity(ctx, activities.TranscodeToAudioAac, common.AudioInput{
				Path:            path,
				Bitrate:         "190k",
				DestinationPath: tempFolder,
			})
		}
	}

	var audioFiles = map[string]string{}
	{
		keys, err := wfutils.GetMapKeysSafely(ctx, audioTasks)
		if err != nil {
			return nil, err
		}
		for _, lang := range keys {
			task := audioTasks[lang]
			var result common.AudioResult
			err = task.Get(ctx, &result)
			if err != nil {
				return nil, err
			}
			audioFiles[lang] = result.OutputPath
		}
	}

	var videoFiles = map[string]string{}
	{
		keys, err := wfutils.GetMapKeysSafely(ctx, videoTasks)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			task := videoTasks[key]
			var result common.VideoResult
			err = task.Get(ctx, &result)
			if err != nil {
				return nil, err
			}
			videoFiles[key] = result.OutputPath
		}
	}

	return &PrepareFilesResult{
		VideoFiles: videoFiles,
		AudioFiles: audioFiles,
	}, nil
}
