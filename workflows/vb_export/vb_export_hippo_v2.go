package vb_export

import (
	"github.com/ansel1/merry/v2"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/bcc-code/bcc-media-flows/services/transcode"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

/*
VBExportToHippoV2
# Requirements

Uses the new HAP transcoding to encode video instead of putting it into a watch folder.
Video: Various resolutions, 25p/50p, HAP Q codec with audio support
Audio: Included in HAP output
*/
func VBExportToHippoV2(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	return exportToHippoHAP(ctx, params, transcode.HAPFormatHAPQ, "hippo_v2")
}

/*
VBExportToHippoHap
# Requirements

Same as VBExportToHippoV2, but encodes to the plain HAP format instead of HAP Q.
Video: Various resolutions, 25p/50p, HAP codec with audio support
Audio: Included in HAP output
*/
func VBExportToHippoHap(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	return exportToHippoHAP(ctx, params, transcode.HAPFormatHAP, "hippo_hap")
}

func exportToHippoHAP(ctx workflow.Context, params VBExportChildWorkflowParams, format transcode.HAPFormat, flowName string) (*VBExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting VBExport to Hippo", "flow", flowName)

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	isImage, err := wfutils.IsImage(ctx, params.InputFile)
	if err != nil {
		return nil, err
	}

	destExt := params.InputFile.Ext()
	if !isImage {
		destExt = ".mov"
	}

	rcloneDestination := deliveryFolder.Append("Hippo", params.OriginalFilenameWithoutExt+destExt)

	err = wfutils.RcloneWaitForFileGone(ctx, rcloneDestination, telegram.ChatOslofjord, 10)
	if err != nil {
		return nil, err
	}

	hippoOutputDir := params.TempDir.Append(flowName + "_output")
	err = wfutils.CreateFolder(ctx, hippoOutputDir)
	if err != nil {
		return nil, err
	}

	outputFile := hippoOutputDir.Append(params.InputFile.Base())

	if !isImage {
		outputFile = hippoOutputDir.Append(params.InputFile.SetExt("mov").Base())

		if params.AnalyzeResult.FrameRate != 25 && params.AnalyzeResult.FrameRate != 50 {
			return nil, merry.New("Expected 25 or 50 fps input")
		}

		currentVideoFile := params.InputFile
		if params.SubtitleFile != nil {
			// Burn in subtitle first
			videoResult, err := wfutils.Execute(ctx, activities.Video.TranscodeToProResActivity, activities.EncodeParams{
				FilePath:       currentVideoFile,
				OutputDir:      hippoOutputDir,
				Interlace:      false,
				BurnInSubtitle: params.SubtitleFile,
				SubtitleStyle:  params.SubtitleStyle,
				Alpha:          params.AnalyzeResult.HasAlpha,
			}).Result(ctx)
			if err != nil {
				return nil, err
			}
			currentVideoFile = videoResult.OutputPath
		}

		hapResult, err := wfutils.Execute(ctx, activities.Video.TranscodeToHAPActivity, activities.HAPInput{
			FilePath:  currentVideoFile,
			OutputDir: hippoOutputDir,
			Format:    format,
		}).Result(ctx)
		if err != nil {
			return nil, err
		}

		outputFile = hapResult.OutputPath
	} else {
		_ = wfutils.CopyFile(ctx, params.InputFile, outputFile)
	}

	err = wfutils.RcloneCopyFileWithNotifications(ctx, outputFile, rcloneDestination, rclone.PriorityHigh, rcloneNotificationOptions)
	if err != nil {
		return nil, err
	}

	// Intentionally keep the HAP export file in temp storage; it is not deleted after upload.

	notifyExportDone(ctx, params, flowName, outputFile)

	return &VBExportResult{
		ID: params.ParentParams.VXID,
	}, nil
}
