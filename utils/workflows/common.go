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
		StartToCloseTimeout:    time.Hour * 4,
		ScheduleToCloseTimeout: time.Hour * 12,
		HeartbeatTimeout:       time.Minute * 10,
		RetryPolicy: &temporal.RetryPolicy{
			BackoffCoefficient: 2,
			MaximumInterval:    60 * time.Second,
			InitialInterval:    1 * time.Second,
			MaximumAttempts:    10,
		},
	}
}

func GetVXDefaultWorkflowOptions(ctx workflow.Context, vxID string) workflow.ChildWorkflowOptions {
	// Children do not inherit the parent's search attributes, so propagate
	// TriggeredBy explicitly.
	triggeredBy, _ := workflow.GetTypedSearchAttributes(ctx).GetKeyword(TriggeredByKey)
	return workflow.ChildWorkflowOptions{
		RetryPolicy:           &StrictRetryPolicy,
		TaskQueue:             environment.GetWorkerQueue(),
		TypedSearchAttributes: TypedSearchAttributes(vxID, triggeredBy),
	}
}
