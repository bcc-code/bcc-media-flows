package activities

import (
	"context"
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/transcribe"
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

type MergeTranscriptJSONParams struct {
	MergeInput      common.MergeInput
	DestinationPath paths.Path
}

type MergeTranscriptResult struct {
	Path paths.Path
}

// MergeTranscriptJSON is the activity that merges a transcript JSON
// Note that currently Norwegian is hardcoded
func MergeTranscriptJSON(
	ctx context.Context,
	input MergeTranscriptJSONParams,
) (*MergeTranscriptResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "MergeTranscriptJSON")
	log.Info("Starting MergeTranscriptJSON")

	targetFile := input.DestinationPath.Append("merged_transcription.no.json")
	mergedTranscription := transcribe.MergeTranscripts(input.MergeInput)

	marshalled, err := json.Marshal(mergedTranscription)
	if err != nil {
		return nil, err
	}

	_, err = WriteFile(ctx, WriteFileInput{
		Path: targetFile,
		Data: marshalled,
	})
	if err != nil {
		return nil, err
	}

	log.Info("Finished MergeTranscriptJSON")

	return &MergeTranscriptResult{
		Path: targetFile,
	}, nil
}
