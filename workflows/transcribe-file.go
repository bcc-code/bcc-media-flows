package workflows

import (
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"

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
	logger.Info("Starting TranscribeFile")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	tempFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return err
	}

	file, err := paths.Parse(params.File)
	if err != nil {
		return err
	}

	wavFile := common.AudioResult{}
	err = wfutils.Execute(ctx, activities.Audio.TranscodeToAudioWav, common.AudioInput{
		Path:            file,
		DestinationPath: tempFolder,
	}).Get(ctx, &wavFile)
	if err != nil {
		return err
	}

	destination, err := paths.Parse(params.DestinationPath)
	if err != nil {
		return err
	}

	transcribeOutput := &activities.TranscribeResponse{}
	err = wfutils.Execute(ctx, activities.Transcribe, activities.TranscribeParams{
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
