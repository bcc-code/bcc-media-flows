package workflows

import (
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/environment"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type ExecuteFFmpegInput struct {
	Arguments []string
}

func ExecuteFFmpeg(
	ctx workflow.Context,
	params ExecuteFFmpegInput,
) error {
	logger := workflow.GetLogger(ctx)
	options := workflow.ActivityOptions{
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Minute * 1,
			MaximumAttempts: 10,
			MaximumInterval: time.Hour * 1,
		},
		StartToCloseTimeout:    time.Hour * 4,
		ScheduleToCloseTimeout: time.Hour * 48,
		HeartbeatTimeout:       time.Minute * 1,
		TaskQueue:              environment.GetTranscodeQueue(),
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	logger.Info("Starting ExecuteFFmpeg")

	err := workflow.ExecuteActivity(ctx, activities.ExecuteFFmpeg, activities.ExecuteFFmpegInput{
		Arguments: params.Arguments,
	}).Get(ctx, nil)

	if err != nil {
		return err
	}

	return err
}
