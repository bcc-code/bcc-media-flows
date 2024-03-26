package vb_export

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

/*
VBExportToBStage
# Requirements

Container: MOV/MXF
Video: 1080p50, ProRes 422
Audio: PCM, 48kHz, 24Bit
Audio loudness: -23 dB LUFS
Audio tracks:
- Stream1, Track 1: PGM left
- Stream1, Track 2: PGM right
- Stream1, Track 3-16: Timecode/Multitrack Audio (optional)
*/
func VBExportToBStage(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportToBStage")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	bStageOutputDir := params.TempDir.Append("b-stage_output")
	err := wfutils.CreateFolder(ctx, bStageOutputDir)
	if err != nil {
		return nil, err
	}

	filePath := params.InputFile

	isImage, err := wfutils.IsImage(ctx, params.InputFile)
	if err != nil {
		return nil, err
	}
	if !isImage {
		videoResult, err := wfutils.Execute(ctx, activities.Video.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:       params.InputFile,
			OutputDir:      bStageOutputDir,
			Resolution:     "1920x1080",
			FrameRate:      50,
			Interlace:      false,
			BurnInSubtitle: params.SubtitleFile,
			SubtitleStyle:  params.SubtitleStyle,
			Alpha:          false,
		}).Result(ctx)
		if err != nil {
			return nil, err
		}
		filePath = videoResult.OutputPath
	}

	rcloneDestination := deliveryFolder.Append("B-Stage", params.OriginalFilenameWithoutExt+filePath.Ext())

	err = wfutils.RcloneWaitForFileGone(ctx, rcloneDestination, 10)
	if err != nil {
		return nil, err
	}

	err = wfutils.RcloneCopyFile(ctx, filePath, rcloneDestination)
	if err != nil {
		return nil, err
	}

	notifyExportDone(ctx, params, "bstage")

	return &VBExportResult{
		ID: params.ParentParams.VXID,
	}, nil
}
