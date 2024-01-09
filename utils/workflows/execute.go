package wfutils

import (
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/environment"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ExecuteWithQueue executes the specified activity with the correct task queue
func ExecuteWithQueue(ctx workflow.Context, activity any, params ...any) workflow.Future {
	options := workflow.GetActivityOptions(ctx)
	options.TaskQueue = activities.GetQueueForActivity(activity)

	switch options.TaskQueue {
	case environment.GetWorkerQueue():
		if options.RetryPolicy == nil {
			options.RetryPolicy = &temporal.RetryPolicy{
				MaximumAttempts: 10,
				InitialInterval: 30 * time.Second,
				MaximumInterval: 60 * time.Minute,
			}
		}
	// usual reason for this failing is invalid files or tweaks to ffmpeg commands
	case environment.GetTranscodeQueue(), environment.GetAudioQueue():
		if options.RetryPolicy == nil {
			options.RetryPolicy = &temporal.RetryPolicy{
				MaximumAttempts: 5,
				InitialInterval: 30 * time.Second,
				MaximumInterval: 30 * time.Second,
			}
		}
	}

	ctx = workflow.WithActivityOptions(ctx, options)
	return workflow.ExecuteActivity(ctx, activity, params...)
}
