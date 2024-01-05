package activities

import (
	"context"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"go.temporal.io/sdk/activity"
)

type AnalyzeFileParams struct {
	FilePath paths.Path
}

func AnalyzeFile(ctx context.Context, input AnalyzeFileParams) (*ffmpeg.StreamInfo, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting AnalyzeFileActivity")

	info, err := ffmpeg.GetStreamInfo(input.FilePath.Local())
	if err != nil {
		return nil, err
	}
	return &info, nil
}
