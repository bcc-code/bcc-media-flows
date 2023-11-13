package export

import (
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/environment"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"go.temporal.io/sdk/workflow"
)

type PrepareFilesParams struct {
	OutputPath    paths.Path
	VideoFile     paths.Path
	WatermarkPath *paths.Path
	AudioFiles    map[string]paths.Path
}

type PrepareFilesResult struct {
	VideoFiles map[quality]paths.Path
	AudioFiles map[string]paths.Path
}

func getVideoQualities(videoFilePath, outputDir paths.Path, watermarkPath *paths.Path) map[quality]common.VideoInput {
	return map[quality]common.VideoInput{
		r1080p: {
			Path:            videoFilePath,
			DestinationPath: outputDir,
			WatermarkPath:   watermarkPath,
			Width:           1920,
			Height:          1080,
			Bitrate:         "6M",
			BufferSize:      "2M",
		},
		r720p: {
			Path:            videoFilePath,
			DestinationPath: outputDir,
			WatermarkPath:   watermarkPath,
			Width:           1280,
			Height:          720,
			Bitrate:         "3M",
		},
		r540p: {
			Path:            videoFilePath,
			DestinationPath: outputDir,
			WatermarkPath:   watermarkPath,
			Width:           960,
			Height:          540,
			Bitrate:         "1900k",
		},
		r360p: {
			Path:            videoFilePath,
			DestinationPath: outputDir,
			WatermarkPath:   watermarkPath,
			Width:           640,
			Height:          360,
			Bitrate:         "980k",
		},
		r270p: {
			Path:            videoFilePath,
			DestinationPath: outputDir,
			WatermarkPath:   watermarkPath,
			Width:           480,
			Height:          270,
			Bitrate:         "610k",
		},
		r180p: {
			Path:            videoFilePath,
			DestinationPath: outputDir,
			WatermarkPath:   watermarkPath,
			Width:           320,
			Height:          180,
			Bitrate:         "320k",
		},
	}
}

func PrepareFiles(ctx workflow.Context, params PrepareFilesParams) (*PrepareFilesResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting PrepareFiles")

	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	ctx = workflow.WithTaskQueue(ctx, environment.GetTranscodeQueue())

	var videoTasks = map[quality]workflow.Future{}
	{
		qualities := getVideoQualities(params.VideoFile, params.OutputPath, params.WatermarkPath)

		keys, err := wfutils.GetMapKeysSafely(ctx, qualities)
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			input := qualities[key]
			videoTasks[key] = wfutils.ExecuteWithQueue(ctx, activities.TranscodeToVideoH264, input)
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
			audioTasks[lang] = wfutils.ExecuteWithQueue(ctx, activities.TranscodeToAudioAac, common.AudioInput{
				Path:            path,
				Bitrate:         "190k",
				DestinationPath: params.OutputPath,
			})
		}
	}

	var audioFiles = map[string]paths.Path{}
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

	var videoFiles = map[quality]paths.Path{}
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
