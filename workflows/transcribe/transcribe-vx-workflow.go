package transcribe

import (
	"fmt"
	"time"

	"github.com/bcc-code/bccm-flows/activities/transcribe"
	"github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/davecgh/go-spew/spew"
	"github.com/samber/lo"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// TranscribeWorkflowInput is the input to the TranscribeWorkflow
type TranscribeVXWorkflowInput struct {
	Language        string
	DestinationPath string
	VXID            string
}

// TranscribeWorkflow is the workflow that transcribes a video
func TranscribeVXWorkflow(
	ctx workflow.Context,
	params TranscribeVXWorkflowInput,
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

	logger.Info("Starting TranscribeWorkflow")

	shapes := &vidispine.GetFileFromVXResult{}
	err := workflow.ExecuteActivity(ctx, vidispine.GetFileFromVXActivity, vidispine.GetFileFromVXParams{
		Tags: []string{"lowres", "lowres_watermarked", "lowaudio", "original"},
		VXID: params.VXID,
	}).Get(ctx, shapes)

	if err != nil {
		return err
	}

	transcriptionJob := &transcribe.TranscribeActivityResponse{}
	err = workflow.ExecuteActivity(ctx, transcribe.TranscribeActivity, transcribe.TranscribeActivityParams{
		Language:        params.Language,
		File:            shapes.FilePath,
		DestinationPath: params.DestinationPath,
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

	errs := []error{}
	errs = append(errs, importJson.Get(ctx, nil))
	errs = append(errs, importSRT.Get(ctx, nil))

	errs = lo.Filter(errs, func(err error, _ int) bool {
		return err != nil
	})

	if errs != nil {
		spew.Dump(errs)
		return fmt.Errorf("failed to import transcription files: %v", errs)
	}

	return nil
}
