package export

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/bcc-code/mediabank-bridge/log"
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

func getVideosByQuality(videoFilePath, outputDir paths.Path, watermarkPath *paths.Path, resolutions []Resolution) map[resolutionString]common.VideoInput {
	var qualities = map[resolutionString]common.VideoInput{}

	for _, r := range resolutions {
		input := common.VideoInput{
			Path:            videoFilePath,
			DestinationPath: outputDir,
			WatermarkPath:   watermarkPath,
			Width:           r.Width,
			Height:          r.Height,
		}
		if r.Height > 2000 {
			input.Bitrate = "10M"
			input.BufferSize = "2M"
		} else if r.Height > 1000 {
			input.Bitrate = "6M"
			input.BufferSize = "2M"
		} else if r.Height > 700 {
			input.Bitrate = "3M"
		} else if r.Height > 500 {
			input.Bitrate = "1900k"
		} else if r.Height > 300 {
			input.Bitrate = "980k"
		} else if r.Height > 200 {
			input.Bitrate = "610k"
		} else {
			input.Bitrate = "320k"
		}
		qualities[resolutionToString(r)] = input
	}

	return qualities
}

type resolutionString string

func resolutionToString(r Resolution) resolutionString {
	return resolutionString(fmt.Sprintf("%dx%d-%t", r.Width, r.Height, r.File))
}

func resolutionFromString(str resolutionString) Resolution {
	var r Resolution
	_, err := fmt.Sscanf(string(str), "%dx%d-%t", &r.Width, &r.Height, &r.File)
	if err != nil {
		log.L.Error().Err(err).Send()
	}
	return r
}

func doVideoTasks(ctx workflow.Context, selector workflow.Selector, qualities map[resolutionString]common.VideoInput, callback func(f workflow.Future, q Resolution)) ([]resolutionString, error) {
	keys, err := wfutils.GetMapKeysSafely(ctx, qualities)
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		input := qualities[key]
		q := key

		selector.AddFuture(wfutils.Execute(ctx, activities.Video.TranscodeToVideoH264, input).Future, func(f workflow.Future) {
			callback(f, resolutionFromString(q))
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
		selector.AddFuture(wfutils.Execute(ctx, activities.Audio.TranscodeToAudioAac, common.AudioInput{
			Path:            path,
			Bitrate:         "190k",
			DestinationPath: outputPath,
		}).Future, func(f workflow.Future) {
			callback(f, lang)
		})
	}

	return keys, nil
}
