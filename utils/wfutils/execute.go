package wfutils

import (
	"go.temporal.io/sdk/workflow"
)

// ExecuteWithQueue executes the specified activity with the correct task queue
func ExecuteWithQueue(ctx workflow.Context, queue string, activity any, params ...any) workflow.Future {
	options := workflow.GetActivityOptions(ctx)
	options.TaskQueue = queue
	ctx = workflow.WithActivityOptions(ctx, options)
	return workflow.ExecuteActivity(ctx, activity, params...)
}
