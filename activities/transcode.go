package activities

import (
	"context"
	"github.com/bcc-code/bccm-flows/services/transcode"
	"go.temporal.io/sdk/activity"
)

type EncodeParams struct {
	FilePath   string
	OutputDir  string
	Resolution string
	FrameRate  int
	Bitrate    string
}

type EncodeResult struct {
	OutputPath string
}

func TranscodeToProResActivity(ctx context.Context, input EncodeParams) (*EncodeResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToProRes")
	log.Info("Starting TranscodeToProResActivity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	transcodeResult, err := transcode.ProRes(transcode.ProResInput{
		FilePath:   input.FilePath,
		OutputDir:  input.OutputDir,
		FrameRate:  input.FrameRate,
		Resolution: input.Resolution,
	}, progressCallback)
	if err != nil {
		return nil, err
	}

	return &EncodeResult{
		OutputPath: transcodeResult.OutputPath,
	}, nil
}

func TranscodeToH264Activity(ctx context.Context, input EncodeParams) (*EncodeResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToH264")
	log.Info("Starting TranscodeToH264Activity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	transcodeResult, err := transcode.H264(transcode.EncodeInput{
		FilePath:   input.FilePath,
		OutputDir:  input.OutputDir,
		FrameRate:  input.FrameRate,
		Resolution: input.Resolution,
		Bitrate:    input.Bitrate,
	}, progressCallback)
	if err != nil {
		return nil, err
	}

	return &EncodeResult{
		OutputPath: transcodeResult.Path,
	}, nil
}
