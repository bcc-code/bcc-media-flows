package activities

import (
	"context"
	"path/filepath"
	"time"

	"github.com/bcc-code/bccm-flows/services/transcribe"
	"go.temporal.io/sdk/activity"
)

type TranscribeParams struct {
	File            string
	DestinationPath string
	Language        string
}

type TranscribeResponse struct {
	JSONPath string
	SRTPath  string
	TXTPath  string
}

// Transcribe is the activity that transcribes a video
func Transcribe(
	ctx context.Context,
	input TranscribeParams,
) (*TranscribeResponse, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "Transcribe")
	log.Info("Starting Transcribe")

	time.Sleep(time.Second * 10)

	jobData, err := transcribe.DoTranscribe(ctx, input.File, input.DestinationPath, input.Language)

	if err != nil {
		return nil, err
	}
	log.Info("Finished Transcribe")

	fileName := filepath.Base(input.File)
	return &TranscribeResponse{
		JSONPath: filepath.Join(jobData.OutputPath, fileName+".json"),
		SRTPath:  filepath.Join(jobData.OutputPath, fileName+".srt"),
		TXTPath:  filepath.Join(jobData.OutputPath, fileName+".txt"),
	}, nil
}
