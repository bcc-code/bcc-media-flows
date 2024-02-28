package export

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
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

func getVideosByQuality(videoFilePath, outputDir paths.Path, watermarkPath *paths.Path) map[string]common.VideoInput {
	return map[string]common.VideoInput{
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

func doVideoTasks(ctx workflow.Context, selector workflow.Selector, qualities map[string]common.VideoInput, callback func(f workflow.Future, q string)) ([]string, error) {
	keys, err := wfutils.GetMapKeysSafely(ctx, qualities)
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		input := qualities[key]
		q := key

		selector.AddFuture(wfutils.Execute(ctx, activities.TranscodeToVideoH264, input), func(f workflow.Future) {
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
		selector.AddFuture(wfutils.Execute(ctx, activities.TranscodeToAudioAac, common.AudioInput{
			Path:            path,
			Bitrate:         "190k",
			DestinationPath: outputPath,
		}), func(f workflow.Future) {
			callback(f, lang)
		})
	}

	return keys, nil
}
