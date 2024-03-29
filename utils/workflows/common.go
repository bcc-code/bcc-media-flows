package wfutils

import (
	"os"
	"time"

	"github.com/bcc-code/bcc-media-flows/environment"
	"go.temporal.io/sdk/workflow"
)

type ResultOrError[T any] struct {
	Result *T
	Error  error
}

func GetDefaultActivityOptions() workflow.ActivityOptions {
	return workflow.ActivityOptions{
		StartToCloseTimeout:    time.Hour * 4,
		ScheduleToCloseTimeout: time.Hour * 48,
		HeartbeatTimeout:       time.Minute * 1,
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
