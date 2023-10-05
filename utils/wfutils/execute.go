package wfutils

import (
	"github.com/bcc-code/bccm-flows/activities"
	"go.temporal.io/sdk/workflow"
)

// ExecuteWithQueue executes the specified activity with the correct task queue
func ExecuteWithQueue(ctx workflow.Context, activity any, params ...any) workflow.Future {
	options := workflow.GetActivityOptions(ctx)
	options.TaskQueue = activities.GetQueueForActivity(activity)
	ctx = workflow.WithActivityOptions(ctx, options)
	return workflow.ExecuteActivity(ctx, activity, params...)
}
