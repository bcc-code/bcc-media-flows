package miscworkflows

import (
	"fmt"
	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/bcc-code/mediabank-bridge/log"
	"github.com/samber/lo"
	"path/filepath"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"

	"go.temporal.io/sdk/workflow"
)

// TranscodePreviewVXInput is the input to the TranscodePreviewVX workflow
type TranscodePreviewVXInput struct {
	VXID  string
	Delay time.Duration
}

// TranscodePreviewVX is the workflow definition of transcoding a video to preview.
// The workflow should first retrieve the filepath to transcribe from the vx-item,
// then it will generate or use the output folder determined from the workflow run ID
// to output transcoded files, before attaching them to the vx-item as a shape
func TranscodePreviewVX(
	ctx workflow.Context,
	params TranscodePreviewVXInput,
) error {
	if params.Delay > 0 {
		logger := workflow.GetLogger(ctx)
		logger.Info("Delaying workflow execution", "duration", params.Delay)
		workflow.Sleep(ctx, params.Delay)
	}

	logger := workflow.GetLogger(ctx)
	logger.Info("Starting TranscodePreviewVX")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	shapes := &vsactivity.GetFileFromVXResult{}
	err := wfutils.Execute(ctx, activities.Vidispine.GetFileFromVXActivity, vsactivity.GetFileFromVXParams{
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

	allowedExtensions := []string{".mxf", ".mov", ".mp4", ".wav", ".mpg", ".aif", ".mp3"}
	if !lo.Contains(allowedExtensions, strings.ToLower(filepath.Ext(shapes.FilePath.Path))) {
		return fmt.Errorf("unsupported file extension: %s", filepath.Ext(shapes.FilePath.Path))
	}

	previewResponse := &activities.TranscodePreviewResponse{}
	err = wfutils.Execute(ctx, activities.Video.TranscodePreview, activities.TranscodePreviewParams{
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

	err = wfutils.Execute(ctx, activities.Vidispine.ImportFileAsShapeActivity,
		vsactivity.ImportFileAsShapeParams{
			AssetID:  params.VXID,
			FilePath: previewResponse.PreviewFilePath,
			ShapeTag: shapeTag,
		}).Get(ctx, nil)

	for l, p := range previewResponse.AudioPreviewFiles {
		tag := bccmflows.LanguagesByISO[l].MBPreviewTag
		err = wfutils.Execute(ctx, activities.Vidispine.ImportFileAsShapeActivity,
			vsactivity.ImportFileAsShapeParams{
				AssetID:  params.VXID,
				FilePath: p,
				ShapeTag: tag,
			}).Get(ctx, nil)
		if err != nil {
			log.L.Log().Err(err).Msg("Error importing audio preview")
		}
	}

	return err
}
