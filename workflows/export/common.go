package export

import (
	"fmt"

	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

func notifyExportDone(ctx workflow.Context, params VXExportChildWorkflowParams, flow string) {
	_ = notifyTelegramChannel(ctx, fmt.Sprintf("🟩 Export of `%s` finished.\nDestination: `%s`", params.ExportData.Title, flow))
}

func notifyTelegramChannel(ctx workflow.Context, message string) error {
	err := wfutils.NotifyTelegramChannel(ctx, message)
	logger := workflow.GetLogger(ctx)
	if err != nil {
		logger.Error("Failed to notify telegram channel", "error", err)
	}
	return err
}
