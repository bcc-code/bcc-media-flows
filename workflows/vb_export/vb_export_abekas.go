package vb_export

import (
	"fmt"
	"strings"

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

	analyzeResult, err := wfutils.Execute(ctx, activities.Audio.AnalyzeFile, activities.AnalyzeFileParams{
		FilePath: params.InputFile,
	}).Result(ctx)
	if err != nil {
		return nil, err
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
		Resolution:     "1920x1080",
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

	err = wfutils.Execute(ctx, activities.Util.RcloneCopyFile, activities.RcloneFileInput{
		Source:      videoResult.OutputPath,
		Destination: deliveryFolder.Append("Abekas-AVCI", params.OriginalFilenameWithoutExt+extraFileName+videoResult.OutputPath.Ext()),
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	notifyExportDone(ctx, params, "abekas")

	return &VBExportResult{
		ID: params.ParentParams.VXID,
	}, nil
}
