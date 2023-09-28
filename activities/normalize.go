package activities

import (
	"context"

	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"github.com/bcc-code/bccm-flows/services/transcode"
	"go.temporal.io/sdk/activity"
)

type AnalyzeEBUR128Params struct {
	FilePath       string
	TargetLoudness float64
}

func AnalyzeEBUR128Activity(ctx context.Context, input AnalyzeEBUR128Params) (*common.AnalyzeEBUR128Result, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "AnalyzeEBUR128")
	log.Info("Starting AnalyzeEBUR128Activity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	analyzeResult, err := ffmpeg.AnalyzeEBUR128(input.FilePath, progressCallback)
	if err != nil {
		return nil, err
	}

	out := &common.AnalyzeEBUR128Result{
		IntegratedLoudness:  analyzeResult.InputIntegratedLoudness,
		TruePeak:            analyzeResult.InputTruePeak,
		LoudnessRange:       analyzeResult.InputLoudnessRange,
		SuggestedAdjustment: 0.0,
	}

	// The suggested adjustmnet attempts to hit the target loudness
	// but never suggests above -0.9 dBTP. This means it may suggest a
	// negative adjustment if the input according to TP mesaurements is already too loud,
	// event if the integrated loudness is below the target.
	out.SuggestedAdjustment = input.TargetLoudness - analyzeResult.InputIntegratedLoudness

	if analyzeResult.InputTruePeak+out.SuggestedAdjustment > -0.9 {
		out.SuggestedAdjustment = -0.9 - analyzeResult.InputTruePeak
	}

	return out, nil
}

type LinearAdjustAudioParams struct {
	InFilePath  string
	OutFilePath string
	Adjustment  float64
}

func LinearAdjustAudioActivity(ctx context.Context, input *LinearAdjustAudioParams) (*common.AudioResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "LinearAdjustAudio")
	log.Info("Starting LinearAdjustAudioActivity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	return transcode.LinearNormalizeAudio(common.AudioInput{
		Path:            input.InFilePath,
		DestinationPath: input.OutFilePath,
	}, input.Adjustment, progressCallback)
}
