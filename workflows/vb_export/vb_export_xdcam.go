package vb_export

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/utils"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

func VBExportToXDCAM(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	return runVBExportChild(ctx, params, vbExportDestination{
		destination: DestinationXDCAM,
		ext:         ".mxf",
		transcode: func(ctx workflow.Context, params VBExportChildWorkflowParams, outputDir paths.Path) (paths.Path, error) {
			res, err := wfutils.Execute(ctx, activities.Video.TranscodeToXDCAMActivity, activities.EncodeParams{
				FilePath:       params.InputFile,
				OutputDir:      outputDir,
				Resolution:     utils.Resolution1080,
				FrameRate:      25,
				Interlace:      true,
				Bitrate:        "50M",
				BurnInSubtitle: params.SubtitleFile,
				SubtitleStyle:  params.SubtitleStyle,
			}).Result(ctx)
			if err != nil {
				return paths.Path{}, err
			}

			return res.OutputPath, nil
		},
	})
}
