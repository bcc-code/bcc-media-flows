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

	selector := workflow.NewSelector(ctx)

	qualities := getVideoQualities(params.VideoFile, params.OutputPath, params.WatermarkPath)

	var videoFiles = map[quality]paths.Path{}
	videoKeys, err := startVideoTasks(ctx, selector, qualities, func(f workflow.Future, q quality) {
		var result common.VideoResult
		err := f.Get(ctx, &result)
		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to get video result", "error", err)
			return
		}
		videoFiles[q] = result.OutputPath
	})

	if err != nil {
		return nil, err
	}

	var audioFiles = map[string]paths.Path{}
	audioKeys, err := startAudioTasks(ctx, selector, params.AudioFiles, params.OutputPath, func(f workflow.Future, l string) {
		var result common.AudioResult
		err := f.Get(ctx, &result)
		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to get video result", "error", err)
			return
		}
		audioFiles[l] = result.OutputPath
	})
	if err != nil {
		return nil, err
	}

	for range audioKeys {
		selector.Select(ctx)
	}
	for range videoKeys {
		selector.Select(ctx)
	}

	return &PrepareFilesResult{
		VideoFiles: videoFiles,
		AudioFiles: audioFiles,
	}, nil
}

func startVideoTasks(ctx workflow.Context, selector workflow.Selector, qualities map[quality]common.VideoInput, callback func(f workflow.Future, q quality)) ([]quality, error) {
	keys, err := wfutils.GetMapKeysSafely(ctx, qualities)
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		input := qualities[key]
		q := key

		selector.AddFuture(wfutils.ExecuteWithQueue(ctx, activities.TranscodeToVideoH264, input), func(f workflow.Future) {
			callback(f, q)
		})
	}

	return keys, nil
}

func startAudioTasks(ctx workflow.Context, selector workflow.Selector, audioFiles map[string]paths.Path, outputPath paths.Path, callback func(f workflow.Future, l string)) ([]string, error) {
	keys, err := wfutils.GetMapKeysSafely(ctx, audioFiles)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		path := audioFiles[key]
		lang := key
		selector.AddFuture(wfutils.ExecuteWithQueue(ctx, activities.TranscodeToAudioAac, common.AudioInput{
			Path:            path,
			Bitrate:         "190k",
			DestinationPath: outputPath,
		}), func(f workflow.Future) {
			callback(f, lang)
		})
	}

	return keys, nil
}
