package workflows

import (
	"time"

	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/environment"
	"github.com/bcc-code/bccm-flows/paths"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// SubtitleBurnInFileInput is the input to the SubtitleBurnInFile workflow
type SubtitleBurnInFileInput struct {
	VideoFilePath    string
	SubtitleFilePath string
}

// SubtitleBurnInFile can be used to test the transcode activity locally where you have no access to vidispine
// or would like to avoid writing to vidispine. Output folder will be set to the same as where the file is originated.
func SubtitleBurnInFile(
	ctx workflow.Context,
	params SubtitleBurnInFileInput,
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

	logger.Info("Starting SubtitleBurnInFile")

	videoFile, err := paths.Parse(params.VideoFilePath)
	if err != nil {
		return err
	}

	subtitleFile, err := paths.Parse(params.SubtitleFilePath)
	if err != nil {
		return err
	}

	previewResponse := &activities.SubtitleBurnInOutput{}
	err = workflow.ExecuteActivity(ctx, activities.TranscodePreview, activities.SubtitleBurnInInput{
		VideoFile:    videoFile,
		SubtitleFile: subtitleFile,
		OutputPath:   videoFile.Dir(),
	}).Get(ctx, previewResponse)

	if err != nil {
		return err
	}

	return err
}
