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
	}).Get(ctx, nil)

	if err != nil {
		return err
	}

	return err
}
