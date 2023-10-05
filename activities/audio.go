package activities

import (
	"context"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/transcode"
	"go.temporal.io/sdk/activity"
)

func TranscodeToAudioAac(ctx context.Context, input common.AudioInput) (*common.AudioResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToAudioAac")
	log.Info("Starting TranscodeToAudioAacActivity")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	result, err := transcode.AudioAac(input, progressCallback)
	if err != nil {
		return nil, err
	}
	return result, nil
}
