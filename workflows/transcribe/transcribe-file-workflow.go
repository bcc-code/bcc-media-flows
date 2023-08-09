package transcribe

import (
	"time"

	"github.com/bcc-code/bccm-flows/activities/transcribe"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// TranscribeWorkflowInput is the input to the TranscribeWorkflow
type TranscribeFileWorkflowInput struct {
	Language        string
	File            string
	DestinationPath string
}

// TranscribeWorkflow is the workflow that transcribes a video
func TranscribeWorkflow(
	ctx workflow.Context,
	params TranscribeFileWorkflowInput,
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

	logger.Info("Starting TranscribeWorkflow")

	x := workflow.ExecuteActivity(ctx, transcribe.TranscribeActivity, transcribe.TranscribeActivityParams{
		Language:        params.Language,
		File:            params.File,
		DestinationPath: params.DestinationPath,
	}).Get(ctx, nil)

	return x
}
