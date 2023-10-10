package workflows

import (
	"time"

	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// TranscribeFileInput is the input to the TranscribeFile
type TranscribeFileInput struct {
	Language        string
	File            string
	DestinationPath string
}

// TranscribeFile is the workflow that transcribes a video
func TranscribeFile(
	ctx workflow.Context,
	params TranscribeFileInput,
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
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	options.TaskQueue = utils.GetAudioQueue()
	audioCtx := workflow.WithActivityOptions(ctx, options)

	logger.Info("Starting TranscribeFile")

	tempFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return err
	}

	wavFile := common.AudioResult{}
	workflow.ExecuteActivity(audioCtx, activities.TranscodeToAudioWav, common.AudioInput{
		Path:            params.File,
		DestinationPath: tempFolder,
	}).Get(ctx, &wavFile)

	return workflow.ExecuteActivity(ctx, activities.Transcribe, activities.TranscribeParams{
		Language:        params.Language,
		File:            wavFile.OutputPath,
		DestinationPath: params.DestinationPath,
	}).Get(ctx, nil)
}
