package vb_export

import (
	"fmt"
	"strings"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/utils"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

func VBExportToHyperdeck(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	return runVBExportChild(ctx, params, vbExportDestination{
		destination: DestinationHyperdeck,
		ext:         ".mov",
		transcode:   transcodeToHyperdeckProRes,
	})
}

func transcodeToHyperdeckProRes(ctx workflow.Context, params VBExportChildWorkflowParams, outputDir paths.Path) (paths.Path, error) {
	analyzeResult, err := wfutils.Execute(ctx, activities.Audio.AnalyzeFile, activities.AnalyzeFileParams{
		FilePath: params.InputFile,
	}).Result(ctx)
	if err != nil {
		return paths.Path{}, err
	}

	if analyzeResult.HasAlpha {
		return paths.Path{}, fmt.Errorf("hyperdeck export currently does not support alpha channels")
	}

	fileToTranscode := params.InputFile

	// The prefix catches 5.1, 5.1(side), and any other variation.
	if len(analyzeResult.AudioStreams) == 1 && strings.HasPrefix(analyzeResult.AudioStreams[0].ChannelLayout, "5.1") {
		fileToTranscode = params.TempDir.Append("4mono_" + params.InputFile.Base())
		err = wfutils.Execute(ctx, activities.Audio.Convert51to4Mono, common.AudioInput{
			Path:            params.InputFile,
			DestinationPath: fileToTranscode,
		}).Wait(ctx)
		if err != nil {
			return paths.Path{}, err
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
		return paths.Path{}, err
	}

	if videoResult.OutputPath.Ext() != ".mov" {
		return paths.Path{}, fmt.Errorf("expected Hyperdeck ProRes output to be .mov, got %s", videoResult.OutputPath.Ext())
	}

	return videoResult.OutputPath, nil
}
