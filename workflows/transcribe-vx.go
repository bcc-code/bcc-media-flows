package workflows

import (
	"fmt"
	"os"
	"time"

	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"

	"github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/davecgh/go-spew/spew"
	"go.temporal.io/sdk/temporal"
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

	transcodeOptions := workflow.ActivityOptions{
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Minute * 1,
			MaximumAttempts: 10,
			MaximumInterval: time.Hour * 1,
		},
		StartToCloseTimeout:    time.Hour * 4,
		ScheduleToCloseTimeout: time.Hour * 48,
		HeartbeatTimeout:       time.Minute * 1,
		TaskQueue:              utils.GetAudioQueue(),
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	logger.Info("Starting TranscribeVX")

	shapes := &vidispine.GetFileFromVXResult{}
	err := workflow.ExecuteActivity(ctx, vidispine.GetFileFromVXActivity, vidispine.GetFileFromVXParams{
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

	transcodeCtx := workflow.WithActivityOptions(ctx, transcodeOptions)
	wavFile := common.AudioResult{}
	workflow.ExecuteActivity(transcodeCtx, activities.TranscodeToAudioWav, common.AudioInput{
		Path:            shapes.FilePath,
		DestinationPath: tempFolder,
	}).Get(ctx, &wavFile)

	destinationPath, err := wfutils.GetWorkflowOutputFolder(ctx)
	if err != nil {
		return err
	}

	transcriptionJob := &activities.TranscribeResponse{}
	err = workflow.ExecuteActivity(ctx, activities.Transcribe, activities.TranscribeParams{
		Language:        params.Language,
		File:            wavFile.OutputPath,
		DestinationPath: destinationPath,
	}).Get(ctx, transcriptionJob)

	if err != nil {
		return err
	}

	importJson := workflow.ExecuteActivity(ctx, vidispine.ImportFileAsShapeActivity,
		vidispine.ImportFileAsShapeParams{
			AssetID:  params.VXID,
			FilePath: transcriptionJob.JSONPath,
			ShapeTag: "transcription_json",
		})

	importSRT := workflow.ExecuteActivity(ctx, vidispine.ImportFileAsShapeActivity,
		vidispine.ImportFileAsShapeParams{
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

	err = workflow.ExecuteActivity(ctx, vidispine.ImportFileAsSidecarActivity, vidispine.ImportSubtitleAsSidecarParams{
		FilePath: transcriptionJob.SRTPath,
		Language: "no",
		AssetID:  params.VXID,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	txtValue, err := os.ReadFile(transcriptionJob.TXTPath)
	if err != nil {
		return err
	}

	err = workflow.ExecuteActivity(ctx, vidispine.SetVXMetadataFieldActivity, vidispine.SetVXMetadataFieldParams{
		VXID:  params.VXID,
		Key:   transcriptionMetadataFieldName,
		Value: string(txtValue),
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}
