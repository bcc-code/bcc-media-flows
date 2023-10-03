package wfutils

import (
	"fmt"
	vsactivity "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
	"go.temporal.io/sdk/workflow"
	"time"
)

func WaitForVidispineJob(ctx workflow.Context, jobID string) error {
	for {
		var job vsapi.JobDocument
		err := workflow.ExecuteActivity(ctx, vsactivity.GetJob, vsactivity.WaitForJobCompletionParams{
			JobID: jobID,
		}).Get(ctx, &job)
		if err != nil {
			return err
		}
		if job.Status == "FINISHED" {
			return nil
		}
		if job.Status != "STARTED" && job.Status != "READY" && job.Status != "WAITING" {
			return fmt.Errorf("job failed with status: %s", job.Status)
		}
		err = workflow.Sleep(ctx, time.Second*30)
		if err != nil {
			return err
		}
	}
}

func SetVidispineMeta(ctx workflow.Context, assetID, key, value string) error {
	return workflow.ExecuteActivity(ctx, vsactivity.SetVXMetadataFieldActivity, vsactivity.SetVXMetadataFieldParams{
		VXID:  assetID,
		Key:   key,
		Value: value,
	}).Get(ctx, nil)
}
