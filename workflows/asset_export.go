package workflows

import (
	avidispine "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"go.temporal.io/sdk/workflow"
)

type AssetExportParams struct {
	VXID string
}

type AssetExportResult struct {
}

func AssetExport(ctx workflow.Context, params AssetExportParams) (*AssetExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting AssetExport")

	ctx = workflow.WithActivityOptions(ctx, DefaultActivityOptions)

	var data *vidispine.ExportData

	err := workflow.ExecuteActivity(ctx, avidispine.GetExportDataActivity, avidispine.GetExportDataParams{
		VXID: params.VXID,
	}).Get(ctx, &data)
	if err != nil {
		return nil, err
	}

	logger.Info("Retrieved data from vidispine")

	return nil, nil
}
