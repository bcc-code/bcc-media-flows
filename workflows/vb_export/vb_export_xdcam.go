package vb_export

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/bcc-code/bcc-media-flows/utils"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

func VBExportToXDCAM(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting XDCAM")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	outputDir := params.TempDir.Append("xdcam_output")
	err := wfutils.CreateFolder(ctx, outputDir)
	if err != nil {
		return nil, err
	}

	videoResult, err := wfutils.Execute(ctx, activities.Video.TranscodeToXDCAMActivity, activities.EncodeParams{
		FilePath:       params.InputFile,
		OutputDir:      outputDir,
		Resolution:     utils.Resolution1080,
		FrameRate:      25,
		Interlace:      true,
		Bitrate:        "50M",
		BurnInSubtitle: params.SubtitleFile,
		SubtitleStyle:  params.SubtitleStyle,
	}).Result(ctx)
	if err != nil {
		return nil, err
	}

	extraFileName := ""
	if params.SubtitleFile != nil {
		extraFileName += "_SUB_NOR"
	}

	rcloneDestination := deliveryFolder.Append("XDCAM", params.OriginalFilenameWithoutExt+extraFileName+videoResult.OutputPath.Ext())

	err = wfutils.RcloneWaitForFileGone(ctx, rcloneDestination, telegram.ChatOslofjord, 10)
	if err != nil {
		return nil, err
	}

	err = wfutils.RcloneCopyFileWithNotifications(ctx, videoResult.OutputPath, rcloneDestination, rclone.PriorityHigh, rcloneNotificationOptions)
	if err != nil {
		return nil, err
	}

	notifyExportDone(ctx, params, "xdcam", videoResult.OutputPath)

	return &VBExportResult{
		ID: params.ParentParams.VXID,
	}, nil
}
