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

/*
VBExportToAbekas
# Requirements

Container: MXF
Video: 1080i50, AVC-Intra 100
Audio: PCM, 48kHz, 24Bit
Audio loudness: -23 dB LUFS
Audio tracks:
- Stream1, Track 1: PGM left
- Stream1, Track 2: PGM right
- Stream1, Track 3-16: Timecode/Multitrack Audio (optional)
*/
func VBExportToAbekas(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportToAbekas")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	abekasOutputDir := params.TempDir.Append("abekas_output")
	err := wfutils.CreateFolder(ctx, abekasOutputDir)
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
		rcloneDestination := deliveryFolder.Append("Abekas-AVCI", params.InputFile.Base())

		message := fmt.Sprintf("ℹ️ `%s` has alpha channel, copying directly delivery folder with no transoding", params.InputFile.Base())
		wfutils.SendTelegramText(ctx, telegram.ChatOslofjord, message)

		err = wfutils.RcloneWaitForFileGone(ctx, rcloneDestination, telegram.ChatOslofjord, 10)
		if err != nil {
			return nil, err
		}

		err = wfutils.RcloneCopyFileWithNotifications(ctx, params.InputFile, rcloneDestination, rclone.PriorityHigh, rcloneNotificationOptions)
		if err != nil {
			return nil, err
		}

		notifyExportDone(ctx, params, "abekas", params.InputFile)
		return &VBExportResult{
			ID:    params.ParentParams.VXID,
			Title: params.OriginalFilenameWithoutExt,
		}, nil
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
		}).Get(ctx, nil)
		if err != nil {
			return nil, err
		}
	}

	videoResult, err := wfutils.Execute(ctx, activities.Video.TranscodeToAVCIntraActivity, activities.EncodeParams{
		FilePath:       fileToTranscode,
		OutputDir:      abekasOutputDir,
		Resolution:     utils.Resolution1080,
		FrameRate:      50,
		Interlace:      true,
		BurnInSubtitle: params.SubtitleFile,
		SubtitleStyle:  params.SubtitleStyle,
	}).Result(ctx)
	if err != nil {
		return nil, err
	}

	if videoResult.OutputPath.Ext() != ".mxf" {
		return nil, fmt.Errorf("expected avc intra output to be .mxf, got %s", videoResult.OutputPath.Ext())
	}

	extraFileName := ""
	if params.SubtitleFile != nil {
		extraFileName += "_SUB_NOR"
	}

	rcloneDestination := deliveryFolder.Append("Abekas-AVCI", params.OriginalFilenameWithoutExt+extraFileName+videoResult.OutputPath.Ext())

	err = wfutils.RcloneWaitForFileGone(ctx, rcloneDestination, telegram.ChatOslofjord, 10)
	if err != nil {
		return nil, err
	}

	err = wfutils.RcloneCopyFileWithNotifications(ctx, videoResult.OutputPath, rcloneDestination, rclone.PriorityHigh, rcloneNotificationOptions)
	if err != nil {
		return nil, err
	}

	notifyExportDone(ctx, params, "abekas", videoResult.OutputPath)

	return &VBExportResult{
		ID: params.ParentParams.VXID,
	}, nil
}
