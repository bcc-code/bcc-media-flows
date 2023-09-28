package workflows

import (
	"time"

	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
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
		TaskQueue:              utils.GetTranscodeQueue(),
	}

	ctx = workflow.WithActivityOptions(ctx, options)
	out := &NormalizeAudioResult{}

	logger.Info("Starting NormalizeAudio workflow")

	r128Result := &common.AnalyzeEBUR128Result{}
	err := workflow.ExecuteActivity(ctx, activities.AnalyzeEBUR128Activity, activities.AnalyzeEBUR128Params{
		FilePath:       params.FilePath,
		TargetLoudness: params.TargetLUFS,
	}).Get(ctx, r128Result)
	if err != nil {
		return nil, err
	}

	out.InputAnalysis = r128Result
	outputFolder, err := wfutils.GetWorkflowOutputFolder(ctx)
	if err != nil {
		return nil, err
	}

	adjustResult := &common.AudioResult{}
	err = workflow.ExecuteActivity(ctx, activities.AdjustAudioLevelActivity, activities.AdjustAudioLevelParams{
		Adjustment:  r128Result.SuggestedAdjustment,
		InFilePath:  params.FilePath,
		OutFilePath: outputFolder,
	}).Get(ctx, adjustResult)
	if err != nil {
		return nil, err
	}

	out.FilePath = adjustResult.OutputPath

	if params.PerformOutputAnalysis {
		r128Result := &common.AnalyzeEBUR128Result{}
		err := workflow.ExecuteActivity(ctx, activities.AnalyzeEBUR128Activity, activities.AnalyzeEBUR128Params{
			FilePath:       adjustResult.OutputPath,
			TargetLoudness: params.TargetLUFS,
		}).Get(ctx, r128Result)
		if err != nil {
			return nil, err
		}

		out.OutputAnalysis = r128Result
	}

	return out, err
}
