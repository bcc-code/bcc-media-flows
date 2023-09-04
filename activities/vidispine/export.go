package vidispine

import (
	"context"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"go.temporal.io/sdk/activity"
)

type GetExportDataParams struct {
	VXID string
}

func GetExportDataActivity(ctx context.Context, params *GetExportDataParams) (*vidispine.ExportData, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "GetExportDataActivity")
	log.Info("Starting GetExportDataActivity")

	client := getClient()

	data, err := client.GetDataForExport(params.VXID)
	if err != nil {
		return nil, err
	}

	return data, nil
}