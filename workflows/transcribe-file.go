package workflows

import (
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
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

	file := paths.MustParse(params.File)
	destination := paths.MustParse(params.DestinationPath)

	wavFile, err := wfutils.Execute(ctx, activities.Audio.PrepareForTranscription, common.AudioInput{
		Path:            file,
		DestinationPath: tempFolder,
	}).Result(ctx)
	if err != nil {
		return err
	}

	if wavFile == nil {
		// No error and no result means there was no audio to be processed,
		// so we can skip the rest
		return nil
	}

	transcribeOutput := &activities.TranscribeResponse{}
	err = wfutils.Execute(ctx, activities.Util.Transcribe, activities.TranscribeParams{
		Language:        params.Language,
		File:            wavFile.OutputPath,
		DestinationPath: tempFolder,
	}).Get(ctx, transcribeOutput)
	if err != nil || transcribeOutput == nil {
		return err
	}

	_, err = wfutils.MoveToFolder(ctx, transcribeOutput.JSONPath, destination, rclone.PriorityNormal)
	if err != nil {
		return err
	}
	_, err = wfutils.MoveToFolder(ctx, transcribeOutput.SRTPath, destination, rclone.PriorityNormal)
	if err != nil {
		return err
	}
	_, err = wfutils.MoveToFolder(ctx, transcribeOutput.TXTPath, destination, rclone.PriorityNormal)
	return err

}
