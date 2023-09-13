package activities

import (
	"context"
	"fmt"
	"github.com/bcc-code/bccm-flows/services/transcode"
	"go.temporal.io/sdk/activity"
)

type TranscodePreviewParams struct {
	FilePath           string
	DestinationDirPath string
}

type TranscodePreviewResponse struct {
	PreviewFilePath string
	AudioOnly       bool
}

// TranscodePreview is the activity definition for transcoding a video to preview. It only uses the specified filepath
// and output dir to create the necessary files. Requires ffmpeg and ffprobe to be installed on the worker running this.
func TranscodePreview(ctx context.Context, input TranscodePreviewParams) (*TranscodePreviewResponse, error) {
	activity.RecordHeartbeat(ctx, "Transcode Preview")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	result, err := transcode.Preview(transcode.PreviewInput{
		OutputDir: input.DestinationDirPath,
		FilePath:  input.FilePath,
	}, progressCallback)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	return &TranscodePreviewResponse{
		PreviewFilePath: result.LowResolutionPath,
		AudioOnly:       result.AudioOnly,
	}, nil
}
