package workflows

import (
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/utils"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// TranscodePreviewVXInput is the input to the TranscodePreviewVX workflow
type TranscodePreviewVXInput struct {
	VXID string
}

// TranscodePreviewVX is the workflow definition of transcoding a video to preview.
// The workflow should first retrieve the filepath to transcribe from the vx-item,
// then it will generate or use the output folder determined from the workflow run ID
// to output transcoded files, before attaching them to the vx-item as a shape
func TranscodePreviewVX(
	ctx workflow.Context,
	params TranscodePreviewVXInput,
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
		TaskQueue:              utils.GetWorkerQueue(),
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	logger.Info("Starting TranscodePreviewVX")

	shapes := &vidispine.GetFileFromVXResult{}
	err := workflow.ExecuteActivity(ctx, vidispine.GetFileFromVXActivity, vidispine.GetFileFromVXParams{
		Tags: []string{"original"},
		VXID: params.VXID,
	}).Get(ctx, shapes)

	if err != nil {
		return err
	}

	destinationPath, err := getWorkflowOutputFolder(ctx)
	if err != nil {
		return err
	}

	previewResponse := &activities.TranscodePreviewResponse{}
	ctx = workflow.WithTaskQueue(ctx, utils.GetTranscodeQueue())
	err = workflow.ExecuteActivity(ctx, activities.TranscodePreview, activities.TranscodePreviewParams{
		FilePath:           shapes.FilePath,
		DestinationDirPath: destinationPath,
	}).Get(ctx, previewResponse)

	if err != nil {
		return err
	}

	var shapeTag string
	if previewResponse.AudioOnly {
		shapeTag = "lowaudio"
	} else {
		shapeTag = "lowres_watermarked"
	}

	ctx = workflow.WithTaskQueue(ctx, utils.GetWorkerQueue())
	err = workflow.ExecuteActivity(ctx, vidispine.ImportFileAsShapeActivity,
		vidispine.ImportFileAsShapeParams{
			AssetID:  params.VXID,
			FilePath: previewResponse.PreviewFilePath,
			ShapeTag: shapeTag,
		}).Get(ctx, nil)

	return err
}
