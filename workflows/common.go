package workflows

import (
	"github.com/bcc-code/bccm-flows/utils"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"time"
)

func GetDefaultActivityOptions() workflow.ActivityOptions {
	return workflow.ActivityOptions{
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Minute * 1,
			MaximumAttempts: 10,
			MaximumInterval: time.Hour * 1,
		},
		StartToCloseTimeout:    time.Hour * 4,
		ScheduleToCloseTimeout: time.Hour * 48,
		HeartbeatTimeout:       time.Minute * 1,
		TaskQueue:              utils.GetWorkerQueue(),
	}
}

func GetDefaultWorkflowOptions() workflow.ChildWorkflowOptions {
	return workflow.ChildWorkflowOptions{
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Minute * 1,
			MaximumAttempts: 10,
			MaximumInterval: time.Hour * 1,
		},
		TaskQueue: utils.GetWorkerQueue(),
	}
}
