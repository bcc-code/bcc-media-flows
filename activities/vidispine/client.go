package vsactivity

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
)

type Activities struct{}

var Vidispine = Activities{}

func GetClient() vidispine.Client {
	return vsapi.NewClient(os.Getenv("VIDISPINE_BASE_URL"), os.Getenv("VIDISPINE_USERNAME"), os.Getenv("VIDISPINE_PASSWORD"))
}

type WaitForJobCompletionParams struct {
	JobID     string
	SleepTime int
}

type MBJobStatusResult struct {
	JobID  string
	Status string
}

func (a Activities) WaitForJobCompletion(ctx context.Context, params WaitForJobCompletionParams) (*MBJobStatusResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting WaitForJobCompletionActivity")

	vsClient := GetClient()

	sleepTime := time.Second * 30
	if params.SleepTime > 0 {
		sleepTime = time.Second * time.Duration(params.SleepTime)
	}

	for {
		job, err := vsClient.GetJob(params.JobID)
		if err != nil {
			return nil, err
		}
		if job.Status == "FINISHED" {
			return &MBJobStatusResult{params.JobID, job.Status}, nil
		}

		if job.Status != "STARTED" && job.Status != "READY" && job.Status != "WAITING" {
			return &MBJobStatusResult{params.JobID, job.Status}, nil
		}

		activity.RecordHeartbeat(ctx, job)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		time.Sleep(sleepTime)
	}
}

func (a Activities) JobCompleteOrErr(ctx context.Context, params WaitForJobCompletionParams) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting WaitForJobCompletionActivity")

	vsClient := GetClient()

	for {
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

		activity.RecordHeartbeat(ctx, job)
	}
}

type FindJobParams struct {
	ItemID  string
	JobType string
}

func (a Activities) FindJob(ctx context.Context, params FindJobParams) (*vsapi.JobDocument, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting FindJob")

	vsClient := GetClient()
	res, err := vsClient.FindJob(params.ItemID, params.JobType)

	return res, err
}
