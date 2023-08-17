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
}

func TranscodePreview(ctx context.Context, input TranscodePreviewParams) (*TranscodePreviewResponse, error) {
	activity.RecordHeartbeat(ctx, "Transcode Preview")

	currentPercent := 0.0

	progressCallback := func(percent float64) {
		currentPercent = percent
	}

	stopChan := make(chan struct{})
	defer close(stopChan)

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
	}, nil
}
