package vb_export

import (
	"fmt"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/bcc-code/bcc-media-flows/utils"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
	"strings"
)

func VBExportToHyperdeck(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting Hyperdeck")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	outputDir := params.TempDir.Append("hyperdeck_output")
	err := wfutils.CreateFolder(ctx, outputDir)
	if err != nil {
		return nil, err
	}

	analyzeResult, err := wfutils.Execute(ctx, activities.Audio.AnalyzeFile, activities.AnalyzeFileParams{
		FilePath: params.InputFile,
	}).Result(ctx)
	if err != nil {
		return nil, err
	}

	if analyzeResult.HasAlpha {
		return nil, fmt.Errorf("hyperdeck export currently does not support alpha channels")
	}

	fileToTranscode := params.InputFile

	// Check for 5.1 audio
	// Used prefix to catch 5.1, 5.1(side), and any other variations
	if len(analyzeResult.AudioStreams) == 1 && strings.HasPrefix(analyzeResult.AudioStreams[0].ChannelLayout, "5.1") {
		// Convert a one stream 5.1 to 4 mono streams (L, R, Lb, Rb)
		fileToTranscode = params.TempDir.Append("4mono_" + params.InputFile.Base())
		err = wfutils.Execute(ctx, activities.Audio.Convert51to4Mono, common.AudioInput{
			Path:            params.InputFile,
			DestinationPath: fileToTranscode,
		}).Wait(ctx)
		if err != nil {
			return nil, err
		}
	}

	videoResult, err := wfutils.Execute(ctx, activities.Video.TranscodeToHyperdeckProResActivity, activities.EncodeParams{
		FilePath:       fileToTranscode,
		OutputDir:      outputDir,
		Resolution:     utils.Resolution1080,
		FrameRate:      50,
		Interlace:      true,
		BurnInSubtitle: params.SubtitleFile,
		SubtitleStyle:  params.SubtitleStyle,
	}).Result(ctx)
	if err != nil {
		return nil, err
	}

	if videoResult.OutputPath.Ext() != ".mov" {
		return nil, fmt.Errorf("expected Hyperdeck ProRes output to be .mov, got %s", videoResult.OutputPath.Ext())
	}

	extraFileName := ""
	if params.SubtitleFile != nil {
		extraFileName += "_SUB_NOR"
	}

	rcloneDestination := deliveryFolder.Append("Hyperdeck-ProRes", params.OriginalFilenameWithoutExt+extraFileName+videoResult.OutputPath.Ext())

	err = wfutils.RcloneWaitForFileGone(ctx, rcloneDestination, telegram.ChatOslofjord, 10)
	if err != nil {
		return nil, err
	}

	err = wfutils.RcloneCopyFileWithNotifications(ctx, videoResult.OutputPath, rcloneDestination, rclone.PriorityHigh, rcloneNotificationOptions)
	if err != nil {
		return nil, err
	}

	notifyExportDone(ctx, params, "hyperdeck", videoResult.OutputPath)

	return &VBExportResult{
		ID: params.ParentParams.VXID,
	}, nil
}
