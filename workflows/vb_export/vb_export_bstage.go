package vb_export

import (
	"path/filepath"

	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/paths"
	wfutils "github.com/bcc-code/bccm-flows/utils/workflows"
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
		Bitrate:        "100M",
		Interlace:      false,
		BurnInSubtitle: params.SubtitleFile,
		Alpha:          false,
	}).Get(ctx, &videoResult)
	if err != nil {
		return nil, err
	}

	// Mux normalized audio with video
	base := videoResult.OutputPath.Base()
	fileName := base[:len(base)-len(filepath.Ext(base))]
	var muxResult *common.MuxResult
	err = wfutils.ExecuteWithQueue(ctx, activities.TranscodeMuxToSimpleMXF, common.SimpleMuxInput{
		VideoFilePath:   videoResult.OutputPath,
		AudioFilePaths:  []paths.Path{*params.NormalizedAudioFile},
		DestinationPath: params.OutputDir,
		FileName:        fileName,
	}).Get(ctx, &muxResult)

	// Rclone to playout
	/* 	destination := "playout:/dropbox"
	   	err = wfutils.ExecuteWithQueue(ctx, activities.RcloneCopyDir, activities.RcloneCopyDirInput{
	   		Source:      params.OutputDir.Rclone(),
	   		Destination: destination,
	   	}).Get(ctx, nil)
	   	if err != nil {
	   		return nil, err
	   	} */

	return &VBExportResult{
		ID:    params.ParentParams.VXID,
		Title: params.ExportData.SafeTitle,
	}, nil
}
