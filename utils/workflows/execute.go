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

// Waiter is the part of Task that does not mention the result type, so one
// slice can hold tasks returning different things when all the caller does is
// wait for them.
type Waiter interface {
	Wait(ctx workflow.Context) error
}

// FutureResult is Task.Result for a bare workflow.Future, for the places where
// the Task wrapper is not available: selector callbacks are handed the future
// itself, and child workflows are started through the SDK rather than Execute.
//
//workflowcheck:ignore
func FutureResult[TR any](ctx workflow.Context, future workflow.Future) (TR, error) {
	return Task[TR]{Future: future}.Result(ctx)
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
	return executeOnQueue(ctx, activities.GetQueueForActivity(activity), activity, params)
}

// ExecuteWithLowPrioQueue executes the utility activities with the low priority queue
func ExecuteWithLowPrioQueue[T any, TR any](ctx workflow.Context, activity func(context.Context, T) (TR, error), params T) Task[TR] {
	queue := activities.GetQueueForActivity(activity)

	// Compared against the constant, not GetWorkerQueue(): in debug mode that
	// accessor returns the debug queue, and the debug worker polls only that
	// queue, so moving its activities to low-priority would leave them
	// unscheduled.
	if queue == environment.QueueWorker {
		queue = environment.QueueLowPriority
	}

	return executeOnQueue(ctx, queue, activity, params)
}

// executeOnQueue schedules the activity on queue with the options the workflow
// set, and the defaults for whatever it did not.
func executeOnQueue[T any, TR any](
	ctx workflow.Context,
	queue string,
	activity func(context.Context, T) (TR, error),
	params T,
) Task[TR] {
	options := workflow.GetActivityOptions(ctx)
	options.TaskQueue = queue

	if options.RetryPolicy == nil {
		options.RetryPolicy = retryPolicyForQueue(queue)
	}

	ctx = workflow.WithActivityOptions(ctx, activityOptionsWithDefaults(options))

	return Task[TR]{Future: workflow.ExecuteActivity(ctx, activity, params)}
}

// retryPolicyForQueue picks how hard to retry from the kind of work the queue
// carries, for workflows that did not choose a policy themselves.
func retryPolicyForQueue(queue string) *temporal.RetryPolicy {
	return retryPolicyForQueueNames(
		queue,
		environment.GetWorkerQueue(),
		environment.GetTranscodeQueue(),
		environment.GetAudioQueue(),
	)
}

// retryPolicyForQueueNames takes the queue names as arguments so the debug
// arrangement, where all three collapse onto one queue, can be exercised
// without the process-wide QUEUE variable that decides them.
func retryPolicyForQueueNames(queue, worker, transcode, audio string) *temporal.RetryPolicy {
	switch queue {
	// Worker first: in debug mode one worker runs everything and all three names
	// are the debug queue, so this ordering is what keeps the behaviour of the
	// ordered switch it replaces.
	case worker:
		return &LooseRetryPolicy
	// A transcode or audio failure is usually an invalid file or a tweak needed
	// to an ffmpeg command, and neither is fixed by trying ten times.
	case transcode, audio:
		return &StrictRetryPolicy
	// Low priority and live ingest carry the same work as the worker queue.
	default:
		return &LooseRetryPolicy
	}
}
