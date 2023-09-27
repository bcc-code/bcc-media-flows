package workflows

import (
	"time"

	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/utils"
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
	InputAnalysis  *activities.AnalyzeEBUR128Result
	OutputAnalysis *activities.AnalyzeEBUR128Result
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
		TaskQueue:              utils.GetTranscodeQueue(),
	}

	ctx = workflow.WithActivityOptions(ctx, options)
	out := &NormalizeAudioResult{}

	logger.Info("Starting NormalizeAudio workflow")

	r128Result := &activities.AnalyzeEBUR128Result{}
	err := workflow.ExecuteActivity(ctx, activities.AnalyzeEBUR128Activity, activities.AnalyzeEBUR128Params{
		FilePath:       params.FilePath,
		TargetLoudness: params.TargetLUFS,
	}).Get(ctx, r128Result)
	if err != nil {
		return nil, err
	}

	out.InputAnalysis = r128Result

	utils.GetWorkflowTempFolder(ctx)

	adjustResult := &activities.LinearAdjustAudioResult{}
	err = workflow.ExecuteActivity(ctx, activities.LinearAdjustAudioActivity, activities.LinearAdjustAudioParams{
		Adjustment:  r128Result.SuggestedAdjustment,
		InFilePath:  params.FilePath,
		OutFilePath: utils.GetWorkflowOutputFolder(ctx),
	}).Get(ctx, adjustResult)
	if err != nil {
		return nil, err
	}

	out.FilePath = adjustResult.OutFilePath

	if params.PerformOutputAnalysis {
		r128Result := &activities.AnalyzeEBUR128Result{}
		err := workflow.ExecuteActivity(ctx, activities.AnalyzeEBUR128Activity, activities.AnalyzeEBUR128Params{
			FilePath:       adjustResult.OutFilePath,
			TargetLoudness: params.TargetLUFS,
		}).Get(ctx, r128Result)
		if err != nil {
			return nil, err
		}

		out.OutputAnalysis = r128Result
	}

	return out, err
}
