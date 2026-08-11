package export

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/utils"
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

func getVideosByQuality(videoFilePath, outputDir paths.Path, watermarkPath *paths.Path, resolutions []utils.Resolution) map[resolutionString]common.VideoInput {
	var qualities = map[resolutionString]common.VideoInput{}

	for _, r := range resolutions {
		input := common.VideoInput{
			Path:            videoFilePath,
			DestinationPath: outputDir,
			WatermarkPath:   watermarkPath,
			Resolution:      r,
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

func resolutionToString(r utils.Resolution) resolutionString {
	r.EnsureEven()
	return resolutionString(fmt.Sprintf("%dx%d-%t", r.Width, r.Height, r.IsFile))
}

func resolutionFromString(str resolutionString) utils.Resolution {
	var r utils.Resolution

	_, _ = fmt.Sscanf(string(str), "%dx%d-%t", &r.Width, &r.Height, &r.IsFile)
	return r
}

// futureAdder registers a future together with the callback that handles it once
// it resolves. Passing this instead of a raw workflow.Selector lets the caller
// keep an exact count of the futures in flight, which is what the drain loop
// needs: the number of scheduled futures cannot be derived from the input lists,
// because callbacks schedule follow-up work of their own and may bail out early.
type futureAdder func(future workflow.Future, callback func(f workflow.Future))

func doVideoTasks(ctx workflow.Context, addFuture futureAdder, qualities map[resolutionString]common.VideoInput, callback func(f workflow.Future, q utils.Resolution)) error {
	keys, err := wfutils.GetMapKeysSafely(ctx, qualities)
	if err != nil {
		return err
	}

	for _, key := range keys {
		input := qualities[key]
		q := key

		addFuture(wfutils.Execute(ctx, activities.Video.TranscodeToVideoH264, input).Future, func(f workflow.Future) {
			callback(f, resolutionFromString(q))
		})
	}

	return nil
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
