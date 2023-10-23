package ingestworkflows

import (
	vsactivity "github.com/bcc-code/bccm-flows/activities/vidispine"
	"go.temporal.io/sdk/workflow"
)

type importTagResult struct {
	AssetID     string
	ImportJobID string
}

func importTag(ctx workflow.Context, tag, path, title string) (*importTagResult, error) {
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
