package wfutils

import (
	"time"

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
		InitialInterval:        30 * time.Second,
		MaximumInterval:        300 * time.Second,
		NonRetryableErrorTypes: []string{"JOB_FAILED"},
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	return Execute(ctx, activities.Vidispine.JobCompleteOrErr, vsactivity.WaitForJobCompletionParams{
		JobID: jobID,
	}).Wait(ctx)
}

func SetVidispineMeta(ctx workflow.Context, assetID, key, value string) error {
	return Execute(ctx, activities.Vidispine.SetVXMetadataFieldActivity, vsactivity.VXMetadataFieldParams{
		ItemID: assetID,
		Key:    key,
		Value:  value,
	}).Wait(ctx)
}

func SetVidispineMetaInGroup(ctx workflow.Context, assetID, key, value, group string) error {
	return Execute(ctx, activities.Vidispine.SetVXMetadataFieldActivity, vsactivity.VXMetadataFieldParams{
		ItemID:  assetID,
		Key:     key,
		Value:   value,
		GroupID: group,
	}).Wait(ctx)
}

// FindVidispineItemByMetadataField returns the asset ID of the single item whose
// metadata field `name` equals `value`. Returns "" if no item matches. If
// multiple items match, returns the first ID and logs a warning — duplicates
// exist in the wild and we don't want to fail the workflow over them.
func FindVidispineItemByMetadataField(ctx workflow.Context, name, value string) (string, error) {
	ids, err := Execute(ctx, activities.Vidispine.SearchItemsByMetadataField, vsactivity.SearchByMetadataFieldParams{
		Name:  name,
		Value: value,
	}).Result(ctx)
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", nil
	}
	if len(ids) > 1 {
		workflow.GetLogger(ctx).Warn("multiple Vidispine items found for metadata field; using the first",
			"name", name, "value", value, "matches", ids)
	}
	return ids[0], nil
}

func AddVidispineMetaValue(ctx workflow.Context, assetID, key, value string) error {
	return Execute(ctx, activities.Vidispine.AddToVXMetadataFieldActivity, vsactivity.VXMetadataFieldParams{
		ItemID: assetID,
		Key:    key,
		Value:  value,
	}).Wait(ctx)
}
