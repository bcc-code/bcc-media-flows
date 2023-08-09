package transcribe

import (
	"context"
	"path/filepath"
	"time"

	"github.com/bcc-code/bccm-flows/services/transcribe"
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

	jobData, err := transcribe.DoTranscribe(ctx, input.File, input.DestinationPath, input.Language)

	if err != nil {
		return nil, err
	}
	log.Info("Finished TranscribeActivity")

	fileName := filepath.Base(input.File)
	return &TranscribeActivityResponse{
		JSONPath: filepath.Join(jobData.OutputPath, fileName+".json"),
		SRTPath:  filepath.Join(jobData.OutputPath, fileName+".srt"),
	}, nil
}
