package workflows

import (
	"fmt"
	"os"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/utils/workflows"

	"github.com/davecgh/go-spew/spew"
	"go.temporal.io/sdk/workflow"
)

const transcriptionMetadataFieldName = "portal_mf624761"

// TranscribeVXInput is the input to the TranscribeFile
type TranscribeVXInput struct {
	Language string
	VXID     string
}

// TranscribeVX is the workflow that transcribes a video
func TranscribeVX(
	ctx workflow.Context,
	params TranscribeVXInput,
) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting TranscribeVX")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	shapes := &vsactivity.GetFileFromVXResult{}
	err := wfutils.Execute(ctx, vsactivity.GetFileFromVXActivity, vsactivity.GetFileFromVXParams{
		Tags: []string{"lowres", "lowres_watermarked", "lowaudio", "original"},
		VXID: params.VXID,
	}).Get(ctx, shapes)

	if err != nil {
		return err
	}

	tempFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return err
	}

	wavFile := common.AudioResult{}
	err = wfutils.Execute(ctx, activities.Audio.TranscodeToAudioWav, common.AudioInput{
		Path:            shapes.FilePath,
		DestinationPath: tempFolder,
	}).Get(ctx, &wavFile)
	if err != nil {
		return err
	}

	destinationPath, err := wfutils.GetWorkflowAuxOutputFolder(ctx)
	if err != nil {
		return err
	}

	transcriptionJob := &activities.TranscribeResponse{}
	err = wfutils.Execute(ctx, activities.Transcribe, activities.TranscribeParams{
		Language:        params.Language,
		File:            wavFile.OutputPath,
		DestinationPath: destinationPath,
	}).Get(ctx, transcriptionJob)

	if err != nil {
		return err
	}

	importJson := wfutils.Execute(ctx, vsactivity.ImportFileAsShapeActivity,
		vsactivity.ImportFileAsShapeParams{
			AssetID:  params.VXID,
			FilePath: transcriptionJob.JSONPath,
			ShapeTag: "transcription_json",
		})

	importSRT := wfutils.Execute(ctx, vsactivity.ImportFileAsShapeActivity,
		vsactivity.ImportFileAsShapeParams{
			AssetID:  params.VXID,
			FilePath: transcriptionJob.SRTPath,
			ShapeTag: "Transcribed_Subtitle_SRT",
		})

	var errs []error
	err = importJson.Get(ctx, nil)
	if err != nil {
		errs = append(errs, err)
	}
	err = importSRT.Get(ctx, nil)
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		spew.Dump(errs)
		return fmt.Errorf("failed to import transcription files: %v", errs)
	}

	err = wfutils.Execute(ctx, vsactivity.ImportFileAsSidecarActivity, vsactivity.ImportSubtitleAsSidecarParams{
		FilePath: transcriptionJob.SRTPath,
		Language: "no",
		AssetID:  params.VXID,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	txtValue, err := os.ReadFile(transcriptionJob.TXTPath.Local())
	if err != nil {
		return err
	}

	err = wfutils.Execute(ctx, vsactivity.SetVXMetadataFieldActivity, vsactivity.SetVXMetadataFieldParams{
		VXID:  params.VXID,
		Key:   transcriptionMetadataFieldName,
		Value: string(txtValue),
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}
