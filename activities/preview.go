package activities

import (
	"context"
	"fmt"

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
	FilePath           paths.Path
	DestinationDirPath paths.Path
	TempFolderPath     paths.Path
}

func (va VideoActivities) TranscodeGrowingPreview(ctx context.Context, input TranscodeGrowingPreviewParams) (any, error) {
	activity.RecordHeartbeat(ctx, "Transcode Preview")
	err := transcode.GrowingPreview(ctx, transcode.GrowingPreviewInput{
		FilePath:        input.FilePath.Local(),
		DestinationFile: input.DestinationDirPath.Append(input.FilePath.Base()).Local(),
		TempDir:         input.TempFolderPath.Local(),
	})

	return nil, err
}
