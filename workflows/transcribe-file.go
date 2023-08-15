package workflows

import (
	"time"

	"github.com/bcc-code/bccm-flows/activities/transcribe"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// TranscribeFileInput is the input to the TranscribeFile
type TranscribeFileInput struct {
	Language        string
	File            string
	DestinationPath string
}

// TranscribeFile is the workflow that transcribes a video
func TranscribeFile(
	ctx workflow.Context,
	params TranscribeFileInput,
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

	logger.Info("Starting TranscribeFile")

	x := workflow.ExecuteActivity(ctx, transcribe.TranscribeActivity, transcribe.TranscribeActivityParams{
		Language:        params.Language,
		File:            params.File,
		DestinationPath: params.DestinationPath,
	}).Get(ctx, nil)

	return x
}
