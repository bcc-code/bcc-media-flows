package workflows

import (
	"github.com/bcc-code/bccm-flows/activities"
	"path/filepath"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// TranscodeFileInput is the input to the TranscribeFile
type TranscodeFileInput struct {
	FilePath string
}

// TranscodeFile is the workflow that transcribes a video
func TranscodeFile(
	ctx workflow.Context,
	params TranscodeFileInput,
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

	logger.Info("Starting TranscodeFile")

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
