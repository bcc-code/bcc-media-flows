package workflows

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/activities"
	"time"

	"github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/davecgh/go-spew/spew"
	"github.com/samber/lo"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// TranscribeVXInput is the input to the TranscribeFile
type TranscribeVXInput struct {
	Language        string
	DestinationPath string
	VXID            string
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

	ctx = workflow.WithActivityOptions(ctx, options)

	logger.Info("Starting TranscribeFile")

	shapes := &vidispine.GetFileFromVXResult{}
	err := workflow.ExecuteActivity(ctx, vidispine.GetFileFromVXActivity, vidispine.GetFileFromVXParams{
		Tags: []string{"lowres", "lowres_watermarked", "lowaudio", "original"},
		VXID: params.VXID,
	}).Get(ctx, shapes)

	if err != nil {
		return err
	}

	transcriptionJob := &activities.TranscribeResponse{}
	err = workflow.ExecuteActivity(ctx, activities.Transcribe, activities.TranscribeParams{
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

	var errs []error
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
