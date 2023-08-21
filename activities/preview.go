package activities

import (
	"context"
	"fmt"
	"github.com/bcc-code/bccm-flows/services/transcode"
	"go.temporal.io/sdk/activity"
	"time"
)

type TranscodePreviewParams struct {
	FilePath           string
	DestinationDirPath string
}

type TranscodePreviewResponse struct {
	PreviewFilePath string
	AudioOnly       bool
}

func registerProgressCallback(ctx context.Context) (chan struct{}, func(float64)) {
	currentPercent := 0.0

	progressCallback := func(percent float64) {
		currentPercent = percent
	}

	stopChan := make(chan struct{})

	go func() {
		timer := time.NewTicker(time.Second * 5)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				activity.RecordHeartbeat(ctx, currentPercent)
			case <-stopChan:
				return
			}
		}
	}()

	return stopChan, progressCallback
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
		fmt.Println(err)
		return nil, err
	}

	return &TranscodePreviewResponse{
		PreviewFilePath: result.LowResolutionPath,
		AudioOnly:       result.AudioOnly,
	}, nil
}
