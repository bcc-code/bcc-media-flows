package wfutils

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func WaitForVidispineJob(ctx workflow.Context, jobID string) error {
	options := GetDefaultActivityOptions()
	options.RetryPolicy = &temporal.RetryPolicy{
		MaximumAttempts:        240,
		BackoffCoefficient:     1.5,
		InitialInterval:        30,
		MaximumInterval:        300,
		NonRetryableErrorTypes: []string{"JOB_FAILED"},
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	return Execute(ctx, activities.Vidispine.JobCompleteOrErr, vsactivity.WaitForJobCompletionParams{
		JobID: jobID,
	}).Get(ctx, nil)
}

func SetVidispineMeta(ctx workflow.Context, assetID, key, value string) error {
	return Execute(ctx, activities.Vidispine.SetVXMetadataFieldActivity, vsactivity.SetVXMetadataFieldParams{
		VXID:  assetID,
		Key:   key,
		Value: value,
	}).Get(ctx, nil)
}

func SetVidispineMetaInGroup(ctx workflow.Context, assetID, key, value, group string) error {
	return Execute(ctx, activities.Vidispine.SetVXMetadataFieldActivity, vsactivity.SetVXMetadataFieldParams{
		VXID:  assetID,
		Key:   key,
		Value: value,
		Group: group,
	}).Get(ctx, nil)
}

func AddVidispineMetaValue(ctx workflow.Context, assetID, key, value string) error {
	return Execute(ctx, vsactivity.AddVXMetadataFieldValueActivity, vsactivity.SetVXMetadataFieldParams{
		VXID:  assetID,
		Key:   key,
		Value: value,
	}).Get(ctx, nil)
}
