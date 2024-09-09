package wfutils

import (
	"os"
	"time"

	"github.com/bcc-code/bcc-media-flows/environment"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type ResultOrError[T any] struct {
	Result *T
	Error  error
}

func GetDefaultActivityOptions() workflow.ActivityOptions {
	return workflow.ActivityOptions{
		StartToCloseTimeout:    time.Hour * 4,
		ScheduleToCloseTimeout: time.Hour * 12,
		HeartbeatTimeout:       time.Minute * 2,
		RetryPolicy: &temporal.RetryPolicy{
			BackoffCoefficient: 2,
			MaximumInterval:    60 * time.Second,
			InitialInterval:    1 * time.Second,
			MaximumAttempts:    10,
		},
	}
}

func GetVXDefaultWorkflowOptions(vxID string) workflow.ChildWorkflowOptions {
	opts := workflow.ChildWorkflowOptions{
		RetryPolicy: &StrictRetryPolicy,
		TaskQueue:   environment.GetWorkerQueue(),
	}

	if os.Getenv("DEBUG") == "" {
		opts.SearchAttributes = map[string]interface{}{
			"CustomStringField": vxID,
		}
	}

	return opts
}
