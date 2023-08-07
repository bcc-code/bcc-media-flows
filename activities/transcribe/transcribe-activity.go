package transcribe

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
)

type TranscribeActivityParams struct {
	File            string
	DestinationPath string
	Language        string
}

type TranscribeActivityResponse struct {
	JSONPath string
	SRTPath  string
}

// TranscribeActivity is the activity that transcribes a video
func TranscribeActivity(
	ctx context.Context,
	input TranscribeActivityParams,
) (*TranscribeActivityResponse, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscribeActivity")
	log.Info("Starting TranscribeActivity")
	time.Sleep(time.Second * 10)
	log.Info("Finished TranscribeActivity")
	return &TranscribeActivityResponse{}, nil
}
