package workflows

import (
	"github.com/bcc-code/bccm-flows/activities"
	"path/filepath"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// TranscodePreviewFileInput is the input to the TranscribeFile
type TranscodePreviewFileInput struct {
	FilePath string
}

// TranscodePreviewFile is a workflow definition for transcoding a video to a preview
func TranscodePreviewFile(
	ctx workflow.Context,
	params TranscodePreviewFileInput,
) error {

	logger := workflow.GetLogger(ctx)
	options := workflow.ActivityOptions{
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Minute * 1,
			MaximumAttempts: 10,
			MaximumInterval: time.Hour * 1,
		},
		StartToCloseTimeout:    time.Hour * 4,
		ScheduleToCloseTimeout: time.Hour * 48,
		HeartbeatTimeout:       time.Minute * 1,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	logger.Info("Starting TranscodePreviewFile")

	previewResponse := &activities.TranscodePreviewResponse{}
	err := workflow.ExecuteActivity(ctx, activities.TranscodePreview, activities.TranscodePreviewParams{
		FilePath:           params.FilePath,
		DestinationDirPath: filepath.Dir(params.FilePath),
	}).Get(ctx, previewResponse)

	if err != nil {
		return err
	}

	return err
}
