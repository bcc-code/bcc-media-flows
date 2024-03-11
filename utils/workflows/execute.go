package wfutils

import (
	"context"
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

type Future[TR any] struct {
	workflow.Future
}

// Result returns the result of the future
func (f Future[TR]) Result(ctx workflow.Context) (TR, error) {
	var result TR
	err := f.Get(ctx, &result)
	return result, err
}

// Wait waits until the task is done
func (f Future[TR]) Wait(ctx workflow.Context) error {
	return f.Get(ctx, nil)
}

// Execute executes the specified activity with the correct task queue
func Execute[T any, TR any](ctx workflow.Context, activity func(context.Context, T) (TR, error), params T) Future[TR] {
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

	ctx = workflow.WithActivityOptions(ctx, options)
	return Future[TR]{
		workflow.ExecuteActivity(ctx, activity, params),
	}
}

// ExecuteWithLowPrioQueue executes the utility activities with the low priority queue
func ExecuteWithLowPrioQueue[T any, TR any](ctx workflow.Context, activity func(context.Context, T) (TR, error), params T) Future[TR] {
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
	return Future[TR]{
		workflow.ExecuteActivity(ctx, activity, params),
	}
}
