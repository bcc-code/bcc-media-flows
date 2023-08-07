package transcribe

import (
	"time"

	"github.com/bcc-code/bccm-flows/activities/transcribe"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// TranscribeWorkflowInput is the input to the TranscribeWorkflow
type TranscribeWorkflowInput struct {
	Language        string
	File            string
	DestinationPath string
}

// TranscribeWorkflow is the workflow that transcribes a video
func TranscribeWorkflow(
	ctx workflow.Context,
	TranscribeWorkflowInput TranscribeWorkflowInput,
) error {

	logger := workflow.GetLogger(ctx)
	options := workflow.ActivityOptions{
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Minute * 1,
			MaximumAttempts: 5,
			MaximumInterval: time.Minute * 5,
		},
		StartToCloseTimeout: time.Minute * 10,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	logger.Info("Starting TranscribeWorkflow")

	x := workflow.ExecuteActivity(ctx, transcribe.TranscribeActivity, transcribe.TranscribeActivityParams{
		Language:        TranscribeWorkflowInput.Language,
		File:            TranscribeWorkflowInput.File,
		DestinationPath: TranscribeWorkflowInput.DestinationPath,
	}).Get(ctx, nil)

	return x
}
