package miscworkflows

import (
	"github.com/bcc-code/bcc-media-flows/paths"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type NormalizeAudioParams struct {
	FilePath              string
	TargetLUFS            float64
	PerformOutputAnalysis bool
}

type NormalizeAudioResult struct {
	FilePath       string
	InputAnalysis  *common.AnalyzeEBUR128Result
	OutputAnalysis *common.AnalyzeEBUR128Result
}

func NormalizeAudioLevelWorkflow(
	ctx workflow.Context,
	params NormalizeAudioParams,
) (*NormalizeAudioResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting NormalizeAudio workflow")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())
	out := &NormalizeAudioResult{}

	filePath := paths.MustParse(params.FilePath)

	r128Result, err := wfutils.Execute(ctx, activities.Audio.AnalyzeEBUR128Activity, activities.AnalyzeEBUR128Params{
		FilePath:       filePath,
		TargetLoudness: params.TargetLUFS,
	}).Result(ctx)
	if err != nil {
		return nil, err
	}

	out.InputAnalysis = r128Result
	outputFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	// Don't adjust if the suggested adjustment is less than 0.01 Db
	if r128Result.SuggestedAdjustment <= 0.01 {
		adjustResult, err := wfutils.Execute(ctx, activities.Audio.AdjustAudioLevelActivity, activities.AdjustAudioLevelParams{
			Adjustment:  r128Result.SuggestedAdjustment,
			InFilePath:  filePath,
			OutFilePath: outputFolder,
		}).Result(ctx)
		if err != nil {
			return nil, err
		}
		filePath = adjustResult.OutputPath
	}

	out.FilePath = filePath.Local()

	if params.PerformOutputAnalysis {
		r128Result, err := wfutils.Execute(ctx, activities.Audio.AnalyzeEBUR128Activity, activities.AnalyzeEBUR128Params{
			FilePath:       filePath,
			TargetLoudness: params.TargetLUFS,
		}).Result(ctx)
		if err != nil {
			return nil, err
		}

		out.OutputAnalysis = r128Result
	}

	return out, err
}
