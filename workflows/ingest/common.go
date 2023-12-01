package ingestworkflows

import (
	"strconv"

	"github.com/ansel1/merry/v2"
	"github.com/bcc-code/bccm-flows/activities"
	vsactivity "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/services/ingest"
	"github.com/bcc-code/bccm-flows/services/notifications"
	wfutils "github.com/bcc-code/bccm-flows/utils/workflows"
	"github.com/bcc-code/bccm-flows/workflows"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
)

type importTagResult struct {
	AssetID     string
	ImportJobID string
}

func importFileAsTag(ctx workflow.Context, tag string, path paths.Path, title string) (*importTagResult, error) {
	var result vsactivity.CreatePlaceholderResult
	err := workflow.ExecuteActivity(ctx, vsactivity.CreatePlaceholderActivity, vsactivity.CreatePlaceholderParams{
		Title: title,
	}).Get(ctx, &result)
	if err != nil {
		return nil, err
	}
	var job vsactivity.JobResult
	err = workflow.ExecuteActivity(ctx, vsactivity.ImportFileAsShapeActivity, vsactivity.ImportFileAsShapeParams{
		AssetID:  result.AssetID,
		FilePath: path,
		ShapeTag: tag,
	}).Get(ctx, &job)
	if err != nil {
		return nil, err
	}
	return &importTagResult{
		AssetID:     result.AssetID,
		ImportJobID: job.JobID,
	}, nil
}

func createPreviews(ctx workflow.Context, assetIDs []string) error {
	var wfFutures []workflow.ChildWorkflowFuture
	for _, id := range assetIDs {
		wfFutures = append(wfFutures, workflow.ExecuteChildWorkflow(ctx, workflows.TranscodePreviewVX, workflows.TranscodePreviewVXInput{
			VXID: id,
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

func transcribe(ctx workflow.Context, assetIDs []string, language string) error {
	if language == "" {
		language = "no"
	}

	wfFutures := []workflow.ChildWorkflowFuture{}
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
	case OrderFormRawMaterial:
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
	return wfutils.ExecuteWithQueue(ctx, activities.NotifyImportCompleted, activities.NotifyImportCompletedInput{
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
