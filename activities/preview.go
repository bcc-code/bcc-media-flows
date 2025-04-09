package activities

import (
	"context"
	"fmt"
	"time"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/transcode"
	"go.temporal.io/sdk/activity"
)

type TranscodePreviewParams struct {
	FilePath           paths.Path
	DestinationDirPath paths.Path
}

type TranscodePreviewResponse struct {
	PreviewFilePath   paths.Path
	AudioPreviewFiles map[string]paths.Path
	AudioOnly         bool
}

// TranscodePreview is the activity definition for transcoding a video to preview. It only uses the specified filepath
// and output dir to create the necessary files. Requires ffmpeg and ffprobe to be installed on the worker running this.
func (va VideoActivities) TranscodePreview(ctx context.Context, input TranscodePreviewParams) (*TranscodePreviewResponse, error) {
	activity.RecordHeartbeat(ctx, "Transcode Preview")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)
	inputParams := transcode.PreviewInput{
		OutputDir: input.DestinationDirPath.Local(),
		FilePath:  input.FilePath.Local(),
	}

	result, err := transcode.Preview(inputParams, progressCallback)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	result2, err := transcode.AudioPreview(inputParams, progressCallback)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	audioPreviews := map[string]paths.Path{}

	for l, p := range result2.AudioTracks {
		audioPreviews[l] = paths.MustParse(p)
	}

	return &TranscodePreviewResponse{
		PreviewFilePath:   paths.MustParse(result.LowResolutionPath),
		AudioOnly:         result.AudioOnly,
		AudioPreviewFiles: audioPreviews,
	}, nil
}

type TranscodeGrowingPreviewParams struct {
	OriginalFilePath    paths.Path
	DestinationFilePath paths.Path
	TempFolderPath      paths.Path
}

func (va VideoActivities) TranscodeGrowingPreview(ctx context.Context, input TranscodeGrowingPreviewParams) (any, error) {
	activity.RecordHeartbeat(ctx, "Transcode Preview", input)
	err := transcode.GrowingPreview(ctx, transcode.GrowingPreviewInput{
		FilePath:        input.OriginalFilePath.Local(),
		DestinationFile: input.DestinationFilePath.Local(),
		TempDir:         input.TempFolderPath.Local(),
	},
		func(ctx context.Context, duration time.Duration) {
			activity.RecordHeartbeat(ctx, "Transcode Growing Preview", duration)
		},
	)

	return nil, err
}
