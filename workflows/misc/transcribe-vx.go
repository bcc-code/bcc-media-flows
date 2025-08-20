package miscworkflows

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/services/telegram"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/common"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"

	"go.temporal.io/sdk/workflow"
)

const transcriptionMetadataFieldName = "portal_mf624761"

// TranscribeVXInput is the input to the TranscribeFile
type TranscribeVXInput struct {
	Language            string
	VXID                string
	NotificationChannel *telegram.Chat
}

// TranscribeVX is the workflow that transcribes a video
func TranscribeVX(
	ctx workflow.Context,
	params TranscribeVXInput,
) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting TranscribeVX")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	shapes, err := wfutils.Execute(ctx, activities.Vidispine.GetFileFromVXActivity, vsactivity.GetFileFromVXParams{
		Tags: []string{"lowres", "lowres_watermarked", "lowaudio", "original"},
		VXID: params.VXID,
	}).Result(ctx)

	if err != nil {
		return err
	}

	tempFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return err
	}

	prepareResult, err := wfutils.Execute(ctx, activities.Audio.PrepareForTranscription, common.AudioInput{
		Path:            shapes.FilePath,
		DestinationPath: tempFolder,
	}).Result(ctx)
	if err != nil {
		return err
	}

	if !prepareResult.HasAudio {
		return nil
	}

	destinationPath, err := wfutils.GetWorkflowAuxOutputFolder(ctx)
	if err != nil {
		return err
	}

	transcriptionJob, err := wfutils.Execute(ctx, activities.Util.Transcribe, activities.TranscribeParams{
		Language:        params.Language,
		File:            prepareResult.OutputPath,
		DestinationPath: destinationPath,
	}).Result(ctx)

	if err != nil {
		return err
	}

	importJsonJob := wfutils.Execute(ctx, activities.Vidispine.ImportFileAsShapeActivity,
		vsactivity.ImportFileAsShapeParams{
			AssetID:  params.VXID,
			FilePath: transcriptionJob.JSONPath,
			ShapeTag: "transcription_json",
			Replace:  true,
		})

	importSRTJob := wfutils.Execute(ctx, activities.Vidispine.ImportFileAsShapeActivity,
		vsactivity.ImportFileAsShapeParams{
			AssetID:  params.VXID,
			FilePath: transcriptionJob.SRTPath,
			ShapeTag: "Transcribed_Subtitle_SRT",
			Replace:  true,
		})

	var errs []error
	importJsonResult, err := importJsonJob.Result(ctx)
	if err != nil {
		errs = append(errs, err)
	}

	err = importSRTJob.Wait(ctx)
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to import transcription files: %v", errs)
	}

	err = wfutils.WaitForVidispineJob(ctx, importJsonResult.JobID)
	if err != nil {
		return fmt.Errorf("importing of JSON file into Mediabanken failed: %v", errs)
	}

	wfutils.ExecuteIndependently(ctx, activities.Vidispine.ImportFileAsSidecarActivity, vsactivity.ImportSubtitleAsSidecarParams{
		FilePath: transcriptionJob.SRTPath,
		Language: "no",
		AssetID:  params.VXID,
	})

	if params.NotificationChannel != nil {
		wfutils.SendTelegramText(ctx, *params.NotificationChannel, fmt.Sprintf("🟦 Transcription import completed for VXID: %s", params.VXID))
	}

	txtValue, err := wfutils.ReadFile(ctx, transcriptionJob.TXTPath)
	if err != nil {
		return err
	}

	return wfutils.Execute(ctx, activities.Vidispine.SetVXMetadataFieldActivity, vsactivity.VXMetadataFieldParams{
		ItemID: params.VXID,
		Key:    transcriptionMetadataFieldName,
		Value:  string(txtValue),
	}).Wait(ctx)
}
