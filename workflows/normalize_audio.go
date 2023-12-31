package workflows

import (
	"time"

	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/paths"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/temporal"
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
	options := workflow.ActivityOptions{
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Minute * 1,
			MaximumAttempts: 10,
			MaximumInterval: time.Hour * 1,
		},
		StartToCloseTimeout:    time.Hour * 4,
		ScheduleToCloseTimeout: time.Hour * 48,
		HeartbeatTimeout:       time.Minute * 1,
		TaskQueue:              environment.GetWorkerQueue(),
	}

	ctx = workflow.WithActivityOptions(ctx, options)
	out := &NormalizeAudioResult{}

	logger.Info("Starting NormalizeAudio workflow")

	filePath, err := paths.Parse(params.FilePath)
	if err != nil {
		return nil, err
	}

	r128Result := &common.AnalyzeEBUR128Result{}
	err = wfutils.ExecuteWithQueue(ctx, activities.AnalyzeEBUR128Activity, activities.AnalyzeEBUR128Params{
		FilePath:       filePath,
		TargetLoudness: params.TargetLUFS,
	}).Get(ctx, r128Result)
	if err != nil {
		return nil, err
	}

	out.InputAnalysis = r128Result
	outputFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	adjustResult := &common.AudioResult{}

	// Don't adjust if the suggested adjustment is less than 0.01 Db
	if r128Result.SuggestedAdjustment <= 0.01 {
		err = wfutils.ExecuteWithQueue(ctx, activities.AdjustAudioLevelActivity, activities.AdjustAudioLevelParams{
			Adjustment:  r128Result.SuggestedAdjustment,
			InFilePath:  filePath,
			OutFilePath: outputFolder,
		}).Get(ctx, adjustResult)
		if err != nil {
			return nil, err
		}
		filePath = adjustResult.OutputPath
	}

	out.FilePath = filePath.Local()

	if params.PerformOutputAnalysis {
		r128Result := &common.AnalyzeEBUR128Result{}
		err = wfutils.ExecuteWithQueue(ctx, activities.AnalyzeEBUR128Activity, activities.AnalyzeEBUR128Params{
			FilePath:       filePath,
			TargetLoudness: params.TargetLUFS,
		}).Get(ctx, r128Result)
		if err != nil {
			return nil, err
		}

		out.OutputAnalysis = r128Result
	}

	return out, err
}
