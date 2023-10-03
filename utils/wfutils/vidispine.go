package wfutils

import (
	vsactivity "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/workflows"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func WaitForVidispineJob(ctx workflow.Context, jobID string) error {
	options := workflows.GetDefaultActivityOptions()
	options.RetryPolicy = &temporal.RetryPolicy{
		MaximumAttempts:        240,
		BackoffCoefficient:     1.5,
		InitialInterval:        30,
		MaximumInterval:        300,
		NonRetryableErrorTypes: []string{"JOB_FAILED"},
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	return workflow.ExecuteActivity(ctx, vsactivity.JobCompleteOrErr, vsactivity.WaitForJobCompletionParams{
		JobID: jobID,
	}).Get(ctx, nil)
}

func SetVidispineMeta(ctx workflow.Context, assetID, key, value string) error {
	return workflow.ExecuteActivity(ctx, vsactivity.SetVXMetadataFieldActivity, vsactivity.SetVXMetadataFieldParams{
		VXID:  assetID,
		Key:   key,
		Value: value,
	}).Get(ctx, nil)
}
