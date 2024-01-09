package workflows

import (
	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/paths"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/utils/workflows"

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
	options := wfutils.GetDefaultActivityOptions()

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
	wfutils.ExecuteWithQueue(audioCtx, activities.TranscodeToAudioWav, common.AudioInput{
		Path:            file,
		DestinationPath: tempFolder,
	}).Get(ctx, &wavFile)

	destination, err := paths.Parse(params.DestinationPath)
	if err != nil {
		return err
	}

	return wfutils.ExecuteWithQueue(ctx, activities.Transcribe, activities.TranscribeParams{
		Language:        params.Language,
		File:            wavFile.OutputPath,
		DestinationPath: destination,
	}).Get(ctx, nil)
}
