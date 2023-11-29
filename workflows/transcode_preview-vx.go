package workflows

import (
	"path/filepath"
	"time"

	"github.com/bcc-code/bccm-flows/activities"
	vsactivity "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/environment"
	"github.com/bcc-code/bccm-flows/utils/workflows"

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
		TaskQueue:              environment.GetWorkerQueue(),
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	logger.Info("Starting TranscodePreviewVX")

	shapes := &vsactivity.GetFileFromVXResult{}
	err := workflow.ExecuteActivity(ctx, vsactivity.GetFileFromVXActivity, vsactivity.GetFileFromVXParams{
		Tags: []string{"original"},
		VXID: params.VXID,
	}).Get(ctx, shapes)

	if err != nil {
		return err
	}

	destinationPath, err := wfutils.GetWorkflowAuxOutputFolder(ctx)
	if err != nil {
		return err
	}

	switch filepath.Ext(shapes.FilePath.Path) {
	case ".mxf", ".mov", ".mp4", ".wav":
	default:
		return nil
	}

	previewResponse := &activities.TranscodePreviewResponse{}
	ctx = workflow.WithTaskQueue(ctx, environment.GetTranscodeQueue())
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

	ctx = workflow.WithTaskQueue(ctx, environment.GetWorkerQueue())
	err = workflow.ExecuteActivity(ctx, vsactivity.ImportFileAsShapeActivity,
		vsactivity.ImportFileAsShapeParams{
			AssetID:  params.VXID,
			FilePath: previewResponse.PreviewFilePath,
			ShapeTag: shapeTag,
		}).Get(ctx, nil)

	return err
}
