package ingestworkflows

import (
	"strconv"

	"github.com/ansel1/merry/v2"
	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ingest"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/bcc-code/bcc-media-flows/workflows"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
)

type ImportTagResult struct {
	AssetID     string
	ImportJobID string
}

func ImportFileAsTag(ctx workflow.Context, tag string, path paths.Path, title string) (*ImportTagResult, error) {
	var result vsactivity.CreatePlaceholderResult
	err := wfutils.Execute(ctx, activities.Vidispine.CreatePlaceholderActivity, vsactivity.CreatePlaceholderParams{
		Title: title,
	}).Get(ctx, &result)
	if err != nil {
		return nil, err
	}
	var job vsactivity.JobResult
	err = wfutils.Execute(ctx, activities.Vidispine.ImportFileAsShapeActivity, vsactivity.ImportFileAsShapeParams{
		AssetID:  result.AssetID,
		FilePath: path,
		ShapeTag: tag,
	}).Get(ctx, &job)
	if err != nil {
		return nil, err
	}
	return &ImportTagResult{
		AssetID:     result.AssetID,
		ImportJobID: job.JobID,
	}, nil
}

func CreatePreviews(ctx workflow.Context, assetIDs []string) error {
	wfFutures := createPreviewsAsync(ctx, assetIDs)

	for _, f := range wfFutures {
		err := f.Get(ctx, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func createPreviewsAsync(ctx workflow.Context, assetIDs []string) []workflow.ChildWorkflowFuture {
	var wfFutures []workflow.ChildWorkflowFuture
	for _, id := range assetIDs {
		wfFutures = append(wfFutures, workflow.ExecuteChildWorkflow(ctx, workflows.TranscodePreviewVX, workflows.TranscodePreviewVXInput{
			VXID: id,
		}))
	}

	return wfFutures
}

func transcribe(ctx workflow.Context, assetIDs []string, language string) error {
	if language == "" {
		language = "no"
	}

	var wfFutures []workflow.ChildWorkflowFuture
	for _, id := range assetIDs {
		wfFutures = append(wfFutures, workflow.ExecuteChildWorkflow(ctx, workflows.TranscribeVX, workflows.TranscribeVXInput{
			VXID:     id,
			Language: language,
		}))
	}

	for _, f := range wfFutures {
		err := f.Get(ctx, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func getOrderFormFilename(orderForm OrderForm, file paths.Path, props ingest.JobProperty) (string, error) {
	switch orderForm {
	case OrderFormRawMaterial, OrderFormMusic, OrderFormUpload:
		// return filename without extension
		base := file.Base()
		ext := file.Ext()
		return base[:len(base)-len(ext)], nil
	case OrderFormLEDMaterial, OrderFormVBMaster, OrderFormSeriesMaster, OrderFormOtherMaster:
		return masterFilename(props)
	}
	return "", merry.New("Unsupported order form")
}

func notifyImportCompleted(ctx workflow.Context, targets []notifications.Target, jobID int, filesByAssetID map[string]paths.Path) error {
	return wfutils.Execute(ctx, activities.Util.NotifyImportCompleted, activities.NotifyImportCompletedInput{
		Targets: targets,
		Message: notifications.ImportCompleted{
			Title: "Import completed",
			JobID: strconv.Itoa(jobID),
			Files: lo.Map(lo.Entries(filesByAssetID), func(entry lo.Entry[string, paths.Path], _ int) notifications.File {
				return notifications.File{
					VXID: entry.Key,
					Name: entry.Value.Base(),
				}
			}),
		},
	}).Get(ctx, nil)
}
