package wfutils

import (
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
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Minute * 1,
			MaximumAttempts: 10,
			MaximumInterval: time.Hour * 1,
		},
		StartToCloseTimeout:    time.Hour * 4,
		ScheduleToCloseTimeout: time.Hour * 48,
		HeartbeatTimeout:       time.Minute * 1,
		TaskQueue:              environment.GetWorkerQueue(),
	}
}

func GetDefaultWorkflowOptions() workflow.ChildWorkflowOptions {
	return workflow.ChildWorkflowOptions{
		TaskQueue: environment.GetWorkerQueue(),
	}
}

func GetVXDefaultWorkflowOptions(vxID string) workflow.ChildWorkflowOptions {
	return workflow.ChildWorkflowOptions{
		TaskQueue: environment.GetWorkerQueue(),
		SearchAttributes: map[string]interface{}{
			"CustomStringField": vxID,
		},
	}
}
