package ingestworkflows

import (
	"github.com/ansel1/merry/v2"
	vsactivity "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/services/ingest"
	"github.com/bcc-code/bccm-flows/workflows"
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

func transcodeAndTranscribe(ctx workflow.Context, assetIDs []string, language string) error {
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

	if language == "" {
		language = "no"
	}
	wfFutures = []workflow.ChildWorkflowFuture{}
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
