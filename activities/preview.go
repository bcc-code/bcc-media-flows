package activities

import (
	"context"
	"github.com/bcc-code/bccm-flows/services/transcode"
	"go.temporal.io/sdk/activity"
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

	result, err := transcode.Preview(transcode.PreviewInput{
		OutputDir: input.DestinationDirPath,
		FilePath:  input.FilePath,
	})
	if err != nil {
		return nil, err
	}

	return &TranscodePreviewResponse{
		PreviewFilePath: result.LowResolutionPath,
	}, nil
}
