package wfutils

import (
	vsactivity "github.com/bcc-code/bccm-flows/activities/vidispine"
	"go.temporal.io/sdk/workflow"
)

func WaitForVidispineJob(ctx workflow.Context, jobID string) error {
	return workflow.ExecuteActivity(ctx, vsactivity.WaitForJobCompletion, vsactivity.WaitForJobCompletionParams{
		JobID: jobID,
	}).Get(ctx, nil)
}
