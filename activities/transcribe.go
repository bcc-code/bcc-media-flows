package activities

import (
	"context"
	"path/filepath"
	"time"

	"github.com/bcc-code/bccm-flows/paths"

	"github.com/bcc-code/bccm-flows/services/transcribe"
	"go.temporal.io/sdk/activity"
)

type TranscribeParams struct {
	File            paths.Path
	DestinationPath paths.Path
	Language        string
}

type TranscribeResponse struct {
	JSONPath     paths.Path
	SRTPath      paths.Path
	WordsSRTPath paths.Path
	TXTPath      paths.Path
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
		JSONPath:     paths.MustParse(filepath.Join(jobData.OutputPath, fileName+".json")),
		SRTPath:      paths.MustParse(filepath.Join(jobData.OutputPath, fileName+".srt")),
		WordsSRTPath: paths.MustParse(filepath.Join(jobData.OutputPath, fileName+".words.srt")),
		TXTPath:      paths.MustParse(filepath.Join(jobData.OutputPath, fileName+".txt")),
	}, nil
}
