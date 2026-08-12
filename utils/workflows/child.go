package wfutils

import (
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/workflow"
)

// WithAbandonChildOptions returns a context whose child workflows keep running
// after the parent closes, via ParentClosePolicy ABANDON.
//
// This is the only way to hand off work that must outlive the current workflow.
// Activities cannot: they are cancelled when the workflow that scheduled them
// completes, and ParentClosePolicy has no effect on them — setting it and then
// calling ExecuteActivity silently does nothing, which is how sidecar subtitle
// imports were being dropped.
//
// Callers must still wait for the child to actually START, with
// future.GetChildWorkflowExecution().Get(ctx, nil), before completing. If the
// parent closes in the same workflow task that initiated the child, the start is
// never processed and the child is dropped without running.
func WithAbandonChildOptions(ctx workflow.Context) workflow.Context {
	options := workflow.GetChildWorkflowOptions(ctx)
	options.ParentClosePolicy = enums.PARENT_CLOSE_POLICY_ABANDON
	return workflow.WithChildOptions(ctx, options)
}
