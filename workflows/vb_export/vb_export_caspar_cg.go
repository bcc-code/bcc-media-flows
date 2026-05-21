package vb_export

import (
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

// VBExportToCasparCG copies the input file directly to the CasparCG delivery folder without transcoding.
func VBExportToCasparCG(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportToCasparCG")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	rcloneDestination := deliveryFolder.Append("CasparCG", params.OriginalFile.Base())

	err := wfutils.RcloneWaitForFileGone(ctx, rcloneDestination, telegram.ChatOslofjord, 10)
	if err != nil {
		return nil, err
	}

	err = wfutils.RcloneCopyFileWithNotifications(ctx, params.OriginalFile, rcloneDestination, rclone.PriorityHigh, rcloneNotificationOptions)
	if err != nil {
		return nil, err
	}

	notifyExportDone(ctx, params, "caspar-cg", params.OriginalFile)

	return &VBExportResult{
		ID:    params.ParentParams.VXID,
		Title: params.OriginalFilenameWithoutExt,
	}, nil
}
