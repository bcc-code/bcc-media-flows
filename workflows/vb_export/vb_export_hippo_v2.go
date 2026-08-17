package vb_export

import (
	"github.com/ansel1/merry/v2"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
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
	return runVBExportChild(ctx, params, hippo(DestinationHippoV2, transcode.HAPFormatHAPQ))
}

/*
VBExportToHippoHap
# Requirements

Same as VBExportToHippoV2, but encodes to the plain HAP format instead of HAP Q.
Video: Various resolutions, 25p/50p, HAP codec with audio support
Audio: Included in HAP output
*/
func VBExportToHippoHap(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	return runVBExportChild(ctx, params, hippo(DestinationHippoHap, transcode.HAPFormatHAP))
}

func hippo(destination Destination, format transcode.HAPFormat) vbExportDestination {
	return vbExportDestination{
		destination: destination,
		imageAware:  true,
		transcode:   transcodeToHAP(format),
		image:       copyImageToOutputDir,
	}
}

func copyImageToOutputDir(ctx workflow.Context, params VBExportChildWorkflowParams, outputDir paths.Path) (paths.Path, error) {
	outputFile := outputDir.Append(params.InputFile.Base())
	_ = wfutils.CopyFile(ctx, params.InputFile, outputFile)
	return outputFile, nil
}

func transcodeToHAP(format transcode.HAPFormat) transcodeFunc {
	return func(ctx workflow.Context, params VBExportChildWorkflowParams, outputDir paths.Path) (paths.Path, error) {
		if params.AnalyzeResult.FrameRate != 25 && params.AnalyzeResult.FrameRate != 50 {
			return paths.Path{}, merry.New("Expected 25 or 50 fps input")
		}

		currentVideoFile := params.InputFile
		if params.SubtitleFile != nil {
			videoResult, err := wfutils.Execute(ctx, activities.Video.TranscodeToProResActivity, activities.EncodeParams{
				FilePath:       currentVideoFile,
				OutputDir:      outputDir,
				Interlace:      false,
				BurnInSubtitle: params.SubtitleFile,
				SubtitleStyle:  params.SubtitleStyle,
				Alpha:          params.AnalyzeResult.HasAlpha,
			}).Result(ctx)
			if err != nil {
				return paths.Path{}, err
			}
			currentVideoFile = videoResult.OutputPath
		}

		hapResult, err := wfutils.Execute(ctx, activities.Video.TranscodeToHAPActivity, activities.HAPInput{
			FilePath:  currentVideoFile,
			OutputDir: outputDir,
			Format:    format,
		}).Result(ctx)
		if err != nil {
			return paths.Path{}, err
		}

		// The HAP export file is deliberately left in temp storage.
		return hapResult.OutputPath, nil
	}
}
