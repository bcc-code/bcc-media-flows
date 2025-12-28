package vb_export

import (
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

// VBExportToRawAbekas copies the input file directly to Abekas-RAW without transcoding.
func VBExportToRawAbekas(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportToRawAbekas")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	rcloneDestination := deliveryFolder.Append("Abekas-RAW", params.InputFile.Base())

	err := wfutils.RcloneWaitForFileGone(ctx, rcloneDestination, telegram.ChatOslofjord, 10)
	if err != nil {
		return nil, err
	}

	err = wfutils.RcloneCopyFileWithNotifications(ctx, params.InputFile, rcloneDestination, rclone.PriorityHigh, rcloneNotificationOptions)
	if err != nil {
		return nil, err
	}

	notifyExportDone(ctx, params, "raw-abekas", params.InputFile)

	return &VBExportResult{
		ID:    params.ParentParams.VXID,
		Title: params.OriginalFilenameWithoutExt,
	}, nil
}
