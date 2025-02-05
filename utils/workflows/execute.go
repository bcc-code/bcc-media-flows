package wfutils

import (
	"context"
	"go.temporal.io/api/enums/v1"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/environment"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var LooseRetryPolicy = temporal.RetryPolicy{
	MaximumAttempts: 10,
	InitialInterval: 30 * time.Second,
	MaximumInterval: 60 * time.Minute,
}

var StrictRetryPolicy = temporal.RetryPolicy{
	MaximumAttempts: 5,
	InitialInterval: 30 * time.Second,
	MaximumInterval: 30 * time.Second,
}

type Task[TR any] struct {
	Future workflow.Future
}

// Result returns the result of the future
func (f Task[TR]) Result(ctx workflow.Context) (TR, error) {
	var result TR
	err := f.Future.Get(ctx, &result)
	return result, err
}

func (f Task[TR]) Get(ctx workflow.Context, valuePtr any) error {
	return f.Future.Get(ctx, valuePtr)
}

// Wait waits until the task is done
func (f Task[TR]) Wait(ctx workflow.Context) error {
	return f.Future.Get(ctx, nil)
}

// Execute executes the specified activity with the correct task queue
func Execute[T any, TR any](ctx workflow.Context, activity func(context.Context, T) (TR, error), params T) Task[TR] {
	options := workflow.GetActivityOptions(ctx)
	options.TaskQueue = activities.GetQueueForActivity(activity)

	switch options.TaskQueue {
	case environment.GetWorkerQueue():
		if options.RetryPolicy == nil {
			options.RetryPolicy = &LooseRetryPolicy
		}
	// usual reason for this failing is invalid files or tweaks to ffmpeg commands
	case environment.GetTranscodeQueue(), environment.GetAudioQueue():
		if options.RetryPolicy == nil {
			options.RetryPolicy = &StrictRetryPolicy
		}
	}

	if options.ScheduleToCloseTimeout == 0 {
		options.ScheduleToCloseTimeout = time.Hour * 3
	}

	ctx = workflow.WithActivityOptions(ctx, options)
	return Task[TR]{
		workflow.ExecuteActivity(ctx, activity, params),
	}
}

// ExecuteIndependently executes the specified activity in such a way that it continues even if the parent workflow completes before it finishes
func ExecuteIndependently[T any, TR any](ctx workflow.Context, activity func(context.Context, T) (TR, error), params T) Task[TR] {
	parentAbandonOptions := workflow.GetChildWorkflowOptions(ctx)
	parentAbandonOptions.ParentClosePolicy = enums.PARENT_CLOSE_POLICY_ABANDON
	ctx = workflow.WithChildOptions(ctx, parentAbandonOptions)

	return Execute(ctx, activity, params)
}

// ExecuteWithLowPrioQueue executes the utility activities with the low priority queue
func ExecuteWithLowPrioQueue[T any, TR any](ctx workflow.Context, activity func(context.Context, T) (TR, error), params T) Task[TR] {
	options := workflow.GetActivityOptions(ctx)

	options.TaskQueue = activities.GetQueueForActivity(activity)
	if options.TaskQueue == environment.QueueWorker {
		options.TaskQueue = environment.QueueLowPriority
	}

	switch options.TaskQueue {
	case environment.GetWorkerQueue():
		if options.RetryPolicy == nil {
			options.RetryPolicy = &LooseRetryPolicy
		}
	// usual reason for this failing is invalid files or tweaks to ffmpeg commands
	case environment.GetTranscodeQueue(), environment.GetAudioQueue():
		if options.RetryPolicy == nil {
			options.RetryPolicy = &StrictRetryPolicy
		}
	}

	ctx = workflow.WithActivityOptions(ctx, options)
	return Task[TR]{
		workflow.ExecuteActivity(ctx, activity, params),
	}
}
