package activities

import (
	"context"
	"github.com/bcc-code/bccm-flows/services/transcode"
	"go.temporal.io/sdk/activity"
)

type TranscodeToProResParams struct {
	FilePath  string
	OutputDir string
}

type TranscodeToProResResponse struct {
	OutputPath string
}

func TranscodeToProResActivity(ctx context.Context, input TranscodeToProResParams) (*TranscodeToProResResponse, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToProRes")
	log.Info("Starting TranscodeToProResActivity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	transcodeResult, err := transcode.ProRes(transcode.ProResInput{
		FilePath:  input.FilePath,
		OutputDir: input.OutputDir,
	}, progressCallback)
	if err != nil {
		return nil, err
	}

	return &TranscodeToProResResponse{
		OutputPath: transcodeResult.OutputPath,
	}, nil
}
