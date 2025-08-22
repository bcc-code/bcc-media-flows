package miscworkflows

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type ExecuteFFmpegInput struct {
	Arguments []string
}

// ExecuteFFmpeg executes the ffmpeg command with the given arguments
// Provides a live progress report of the ffmpeg command
func ExecuteFFmpeg(
	ctx workflow.Context,
	params ExecuteFFmpegInput,
) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExecuteFFmpeg")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	err := wfutils.Execute(ctx, activities.Video.ExecuteFFmpeg, activities.ExecuteFFmpegInput{
		Arguments: params.Arguments,
	}).Wait(ctx)

	return err
}

func ExecuteFFmpegDump(
	ctx workflow.Context,
	params ExecuteFFmpegInput,
) (string, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExecuteFFmpegDump")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	var out string
	err := wfutils.Execute(ctx, activities.Video.ExecuteFFmpegDump,
		activities.ExecuteFFmpegInput{Arguments: params.Arguments},
	).Get(ctx, &out)

	return out, err
}
