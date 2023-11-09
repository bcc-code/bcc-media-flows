package activities

import (
	"context"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"github.com/bcc-code/bccm-flows/services/transcode"
	"go.temporal.io/sdk/activity"
)

type AnalyzeEBUR128Params struct {
	FilePath       paths.Path
	TargetLoudness float64
}

func AnalyzeEBUR128Activity(ctx context.Context, input AnalyzeEBUR128Params) (*common.AnalyzeEBUR128Result, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "AnalyzeEBUR128")
	log.Info("Starting AnalyzeEBUR128Activity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	analyzeResult, err := ffmpeg.AnalyzeEBUR128(input.FilePath.Local(), progressCallback)
	if err != nil {
		return nil, err
	}

	out := &common.AnalyzeEBUR128Result{
		IntegratedLoudness:  analyzeResult.IntegratedLoudness,
		TruePeak:            analyzeResult.TruePeak,
		LoudnessRange:       analyzeResult.LoudnessRange,
		SuggestedAdjustment: 0.0,
	}

	// The suggested adjustmnet attempts to hit the target loudness
	// but never suggests above -0.9 dBTP. This means it may suggest a
	// negative adjustment if the input according to TP mesaurements is already too loud,
	// event if the integrated loudness is below the target.
	out.SuggestedAdjustment = input.TargetLoudness - analyzeResult.IntegratedLoudness

	if analyzeResult.TruePeak+out.SuggestedAdjustment > -0.9 {
		out.SuggestedAdjustment = -0.9 - analyzeResult.TruePeak
	}

	return out, nil
}

type AdjustAudioLevelParams struct {
	InFilePath  paths.Path
	OutFilePath paths.Path
	Adjustment  float64
}

func AdjustAudioLevelActivity(ctx context.Context, input *AdjustAudioLevelParams) (*common.AudioResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "LinearAdjustAudio")
	log.Info("Starting LinearAdjustAudioActivity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	return transcode.AdjustAudioLevel(common.AudioInput{
		Path:            input.InFilePath,
		DestinationPath: input.OutFilePath,
	}, input.Adjustment, progressCallback)
}

type NormalizeAudioParams struct {
	FilePath              paths.Path
	OutputPath            paths.Path
	TargetLUFS            float64
	PerformOutputAnalysis bool
}

type NormalizeAudioResult struct {
	FilePath       paths.Path
	InputAnalysis  *common.AnalyzeEBUR128Result
	OutputAnalysis *common.AnalyzeEBUR128Result
}

func NormalizeAudioActivity(ctx context.Context, params NormalizeAudioParams) (*NormalizeAudioResult, error) {
	out := &NormalizeAudioResult{}

	r128Result, err := AnalyzeEBUR128Activity(ctx, AnalyzeEBUR128Params{
		FilePath:       params.FilePath,
		TargetLoudness: params.TargetLUFS,
	})
	if err != nil {
		return nil, err
	}

	out.InputAnalysis = r128Result

	adjustResult, err := AdjustAudioLevelActivity(ctx, &AdjustAudioLevelParams{
		Adjustment:  r128Result.SuggestedAdjustment,
		InFilePath:  params.FilePath,
		OutFilePath: params.OutputPath,
	})
	if err != nil {
		return nil, err
	}

	out.FilePath = adjustResult.OutputPath

	if params.PerformOutputAnalysis {
		r128Result, err := AnalyzeEBUR128Activity(ctx, AnalyzeEBUR128Params{
			FilePath:       out.FilePath,
			TargetLoudness: params.TargetLUFS,
		})
		if err != nil {
			return nil, err
		}

		out.OutputAnalysis = r128Result
	}

	return out, err
}
