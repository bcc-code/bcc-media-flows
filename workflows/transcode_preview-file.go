package workflows

import (
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/utils"
	"path/filepath"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// TranscodePreviewFileInput is the input to the TranscodePreviewFile workflow
type TranscodePreviewFileInput struct {
	FilePath string
}

// TranscodePreviewFile can be used to test the transcode activity locally where you have no access to vidispine
// or would like to avoid writing to vidispine. Output folder will be set to the same as where the file is originated.
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
		TaskQueue:              utils.GetTranscodeQueue(),
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
