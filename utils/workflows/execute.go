package wfutils

import (
	"context"
	"reflect"
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
//
//workflowcheck:ignore
func (f Task[TR]) Result(ctx workflow.Context) (TR, error) {
	var result TR
	rv := reflect.ValueOf(&result).Elem()
	var valuePtr interface{}
	if rv.Kind() == reflect.Ptr {
		inner := reflect.New(rv.Type().Elem())
		rv.Set(inner)
		valuePtr = inner.Interface()
	} else {
		valuePtr = &result
	}
	err := f.Future.Get(ctx, valuePtr)
	return result, err
}

func (f Task[TR]) Get(ctx workflow.Context, valuePtr any) error {
	return f.Future.Get(ctx, valuePtr)
}

// Wait waits until the task is done
func (f Task[TR]) Wait(ctx workflow.Context) error {
	return f.Future.Get(ctx, nil)
}

// activityOptionsWithDefaults fills in the timeouts a workflow left unset.
//
// A workflow only has activity options if it called
// workflow.WithActivityOptions, and there is nothing to notice when it did not:
// most activities are reached through helpers like CreateFolder or WriteFile
// rather than through an Execute call the author is looking at. Without this,
// such a workflow schedules activities with no StartToCloseTimeout — so a
// single attempt may consume the entire schedule-to-close budget and never be
// retried — and with a schedule-to-close that differs from the one every
// workflow using GetDefaultActivityOptions gets.
//
// HeartbeatTimeout is deliberately not filled in. A heartbeat timeout on an
// activity that never calls RecordHeartbeat turns a slow success into a
// certain failure, and this function cannot tell which activities heartbeat.
func activityOptionsWithDefaults(options workflow.ActivityOptions) workflow.ActivityOptions {
	defaults := GetDefaultActivityOptions()

	if options.StartToCloseTimeout == 0 {
		options.StartToCloseTimeout = defaults.StartToCloseTimeout
	}
	if options.ScheduleToCloseTimeout == 0 {
		options.ScheduleToCloseTimeout = defaults.ScheduleToCloseTimeout
	}

	return options
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

	ctx = workflow.WithActivityOptions(ctx, activityOptionsWithDefaults(options))

	return Task[TR]{Future: workflow.ExecuteActivity(ctx, activity, params)}
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

	ctx = workflow.WithActivityOptions(ctx, activityOptionsWithDefaults(options))
	return Task[TR]{
		workflow.ExecuteActivity(ctx, activity, params),
	}
}
