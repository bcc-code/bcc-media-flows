package activities

import (
	"context"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"go.temporal.io/sdk/activity"
)

type AnalyzeFileParams struct {
	FilePath paths.Path
}

type AnalyzeFileResult struct {
	HasAudio bool
	HasVideo bool
}

func AnalyzeFile(ctx context.Context, input AnalyzeFileParams) (*AnalyzeFileResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting AnalyzeFileActivity")

	info, err := ffmpeg.GetStreamInfo(input.FilePath.Local())
	if err != nil {
		return nil, err
	}
	return &AnalyzeFileResult{
		HasAudio: info.HasAudio,
		HasVideo: info.HasVideo,
	}, nil
}
