package vsactivity

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/davecgh/go-spew/spew"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
)

func GetClient() vidispine.Client {
	return vsapi.NewClient(os.Getenv("VIDISPINE_BASE_URL"), os.Getenv("VIDISPINE_USERNAME"), os.Getenv("VIDISPINE_PASSWORD"))
}

type WaitForJobCompletionParams struct {
	JobID string
}

func WaitForJobCompletion(ctx context.Context, params WaitForJobCompletionParams) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting WaitForJobCompletionActivity")

	vsClient := GetClient()

	for {
		job, err := vsClient.GetJob(params.JobID)
		if err != nil {
			return err
		}
		if job.Status == "FINISHED" {
			return nil
		}
		if job.Status != "STARTED" && job.Status != "READY" && job.Status != "WAITING" {
			spew.Dump(job)
			return fmt.Errorf("job failed with status: %s", job.Status)
		}
		activity.RecordHeartbeat(ctx, job)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		time.Sleep(time.Second * 30)
	}
}

func JobCompleteOrErr(ctx context.Context, params WaitForJobCompletionParams) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting WaitForJobCompletionActivity")

	vsClient := GetClient()

	job, err := vsClient.GetJob(params.JobID)
	if err != nil {
		return false, temporal.NewNonRetryableApplicationError("couldn't complete job", "JOB_FAILED", err)
	}
	if job.Status == "FINISHED" {
		return true, nil
	}
	if job.Status != "STARTED" && job.Status != "READY" && job.Status != "WAITING" {
		return false, temporal.NewNonRetryableApplicationError("couldn't complete job", "JOB_FAILED", fmt.Errorf("job failed with status: %s", job.Status), job)
	}

	return false, fmt.Errorf("job not finished yet")
}
