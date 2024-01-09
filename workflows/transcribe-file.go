package workflows

import (
	"time"

	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"

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

	options.TaskQueue = environment.GetAudioQueue()
	audioCtx := workflow.WithActivityOptions(ctx, options)

	logger.Info("Starting TranscribeFile")

	tempFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return err
	}

	file, err := paths.Parse(params.File)
	if err != nil {
		return err
	}

	wavFile := common.AudioResult{}
	workflow.ExecuteActivity(audioCtx, activities.TranscodeToAudioWav, common.AudioInput{
		Path:            file,
		DestinationPath: tempFolder,
	}).Get(ctx, &wavFile)

	destination, err := paths.Parse(params.DestinationPath)
	if err != nil {
		return err
	}

	transcribeOutput := &activities.TranscribeResponse{}
	err = workflow.ExecuteActivity(ctx, activities.Transcribe, activities.TranscribeParams{
		Language:        params.Language,
		File:            wavFile.OutputPath,
		DestinationPath: tempFolder,
	}).Get(ctx, transcribeOutput)

	if err != nil || transcribeOutput == nil {
		return err
	}

	_, err = wfutils.MoveToFolder(ctx, transcribeOutput.JSONPath, destination)
	if err != nil {
		return err
	}
	_, err = wfutils.MoveToFolder(ctx, transcribeOutput.SRTPath, destination)
	if err != nil {
		return err
	}
	_, err = wfutils.MoveToFolder(ctx, transcribeOutput.TXTPath, destination)
	return err

}
