package vb_export

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

/*
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

	var videoResult common.VideoResult
	err = wfutils.Execute(ctx, activities.TranscodeToAVCIntraActivity, activities.EncodeParams{
		FilePath:       params.InputFile,
		OutputDir:      abekasOutputDir,
		Resolution:     "1920x1080",
		FrameRate:      50,
		Interlace:      true,
		BurnInSubtitle: params.SubtitleFile,
	}).Get(ctx, &videoResult)
	if err != nil {
		return nil, err
	}

	if videoResult.OutputPath.Ext() != ".mxf" {
		return nil, fmt.Errorf("expected avc intra output to be .mxf, got %s", videoResult.OutputPath.Ext())
	}

	err = wfutils.Execute(ctx, activities.RcloneCopyFile, activities.RcloneFileInput{
		Source:      videoResult.OutputPath,
		Destination: deliveryFolder.Append("Abekas-AVCI", params.OriginalFilenameWithoutExt+videoResult.OutputPath.Ext()),
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &VBExportResult{
		ID: params.ParentParams.VXID,
	}, nil
}
