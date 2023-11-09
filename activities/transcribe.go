package activities

import (
	"context"
	"github.com/bcc-code/bccm-flows/paths"
	"path/filepath"
	"time"

	"github.com/bcc-code/bccm-flows/services/transcribe"
	"go.temporal.io/sdk/activity"
)

type TranscribeParams struct {
	File            paths.Path
	DestinationPath paths.Path
	Language        string
}

type TranscribeResponse struct {
	JSONPath paths.Path
	SRTPath  paths.Path
	TXTPath  paths.Path
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

	jobData, err := transcribe.DoTranscribe(ctx, input.File.Local(), input.DestinationPath.Local(), input.Language)
	if err != nil {
		return nil, err
	}

	log.Info("Finished Transcribe")

	fileName := input.File.Base()
	return &TranscribeResponse{
		JSONPath: paths.MustParsePath(filepath.Join(jobData.OutputPath, fileName+".json")),
		SRTPath:  paths.MustParsePath(filepath.Join(jobData.OutputPath, fileName+".srt")),
		TXTPath:  paths.MustParsePath(filepath.Join(jobData.OutputPath, fileName+".txt")),
	}, nil
}
