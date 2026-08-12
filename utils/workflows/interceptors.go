package wfutils

import (
	"context"
	"time"

	"github.com/bcc-code/bcc-media-flows/analytics"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/workflow"
)

// WORKER INTERCEPTORS

type AnalyticsWorkerInterceptor struct {
	interceptor.WorkerInterceptorBase
}

func (c *AnalyticsWorkerInterceptor) InterceptWorkflow(
	ctx workflow.Context,
	next interceptor.WorkflowInboundInterceptor,
) interceptor.WorkflowInboundInterceptor {
	return &AnalyticsWorkflowInboundInterceptor{
		WorkflowInboundInterceptorBase: interceptor.WorkflowInboundInterceptorBase{
			Next: next,
		},
	}
}

func (c *AnalyticsWorkerInterceptor) InterceptActivity(
	ctx context.Context,
	next interceptor.ActivityInboundInterceptor,
) interceptor.ActivityInboundInterceptor {
	return &AnalyticsActivityInboundInterceptor{
		ActivityInboundInterceptorBase: interceptor.ActivityInboundInterceptorBase{
			Next: next,
		},
	}
}

// WORKFLOW INTERCEPTOR

type AnalyticsWorkflowInboundInterceptor struct {
	interceptor.WorkflowInboundInterceptorBase
}

func (c *AnalyticsWorkflowInboundInterceptor) ExecuteWorkflow(
	ctx workflow.Context,
	in *interceptor.ExecuteWorkflowInput,
) (any, error) {
	info := workflow.GetInfo(ctx)

	parent := ""
	if info.ParentWorkflowExecution != nil {
		parent = info.ParentWorkflowExecution.ID
	}

	// Emitting only outside replay keeps one event per workflow: a replayed task
	// re-runs this interceptor from the top, so an unguarded call would report the
	// same workflow started again on every cache eviction.
	if !workflow.IsReplaying(ctx) {
		//workflowcheck:ignore analytics writes to stdout/rudderstack, but only outside replay
		analytics.GetService().WorkflowStarted(info.WorkflowType.Name, info.WorkflowExecution.ID, parent)
	}

	// workflow.Now, not time.Now: the wall clock differs between the original run
	// and a replay, and it measures time spent in this worker rather than the
	// workflow's own elapsed time.
	startTime := workflow.Now(ctx)

	result, err := c.Next.ExecuteWorkflow(ctx, in)

	duration := workflow.Now(ctx).Sub(startTime)
	executionTime := duration.Milliseconds()

	status := "Success"
	if err != nil {
		status = "Failure"
	}

	if !workflow.IsReplaying(ctx) {
		//workflowcheck:ignore analytics writes to stdout/rudderstack, but only outside replay
		analytics.GetService().WorkflowFinished(info.WorkflowType.Name, info.WorkflowExecution.ID, parent, status, executionTime)
	}

	return result, err
}

// ACTIVITY INTERCEPTOR

type AnalyticsActivityInboundInterceptor struct {
	interceptor.ActivityInboundInterceptorBase
}

func (c *AnalyticsActivityInboundInterceptor) ExecuteActivity(
	ctx context.Context, in *interceptor.ExecuteActivityInput,
) (any, error) {
	// Counted here rather than where the activity was scheduled: this runs in
	// the process actually executing it, and the deferred call cannot be
	// skipped the way a workflow coroutine can.
	defer RunningActivities.Started()()

	info := activity.GetInfo(ctx)
	startTime := time.Now()
	workerIdentity, _ := ctx.Value("WorkerIdentity").(string)

	analytics.GetService().ActivityStarted(
		info.ActivityType.Name,
		info.TaskQueue,
		info.WorkflowExecution.ID,
	)

	result, err := c.Next.ExecuteActivity(ctx, in)

	duration := time.Since(startTime)
	executionTime := duration.Milliseconds()

	status := "Success"
	if err != nil {
		status = "Failure"
	}

	analytics.GetService().ActivityFinished(
		info.ActivityType.Name,
		workerIdentity,
		info.TaskQueue,
		info.WorkflowExecution.ID,
		status,
		executionTime,
	)

	return result, err
}
