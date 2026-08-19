package wfutils

import (
	"errors"
	"time"

	"github.com/bcc-code/bcc-media-flows/environment"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type ResultOrError[T any] struct {
	Result *T
	Error  error
}

// CollectChildResults waits for every future in order and keeps one entry per
// child, failed or not, so the caller can report on all of them. onError is
// called as each failure arrives. The returned error joins them.
func CollectChildResults[T any](ctx workflow.Context, futures []workflow.Future, onError func(error)) ([]ResultOrError[T], error) {
	var results []ResultOrError[T]
	var errs []error

	for _, future := range futures {
		var result *T
		err := future.Get(ctx, &result)
		results = append(results, ResultOrError[T]{
			Result: result,
			Error:  err,
		})
		if err != nil {
			errs = append(errs, err)
			if onError != nil {
				onError(err)
			}
		}
	}

	if len(errs) == 0 {
		return results, nil
	}

	return results, errors.Join(errs...)
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
