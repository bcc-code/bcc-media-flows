package vb_export

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

/*
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

	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	bStageOutputDir := params.TempDir.Append("b-stage_output")
	err := wfutils.CreateFolder(ctx, bStageOutputDir)
	if err != nil {
		return nil, err
	}

	var videoResult common.VideoResult
	err = wfutils.ExecuteWithQueue(ctx, activities.TranscodeToProResActivity, activities.EncodeParams{
		FilePath:       params.InputFile,
		OutputDir:      bStageOutputDir,
		Resolution:     "1920x1080",
		FrameRate:      50,
		Interlace:      false,
		BurnInSubtitle: params.SubtitleFile,
		Alpha:          false,
	}).Get(ctx, &videoResult)
	if err != nil {
		return nil, err
	}

	err = wfutils.ExecuteWithQueue(ctx, activities.RcloneCopyFile, activities.RcloneFileInput{
		Source:      videoResult.OutputPath,
		Destination: deliveryFolder.Append("B-Stage", params.OriginalFilenameWithoutExt+videoResult.OutputPath.Ext()),
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &VBExportResult{
		ID: params.ParentParams.VXID,
	}, nil
}
