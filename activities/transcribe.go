package activities

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/transcribe"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
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

// transcribeIdempotencyKey is per attempt: a reused key returns the old job even
// when it FAILED. Run ID, because reruns can reuse a workflow ID
func transcribeIdempotencyKey(ctx context.Context) string {
	info := activity.GetInfo(ctx)
	return fmt.Sprintf("%s/%s/%d", info.WorkflowExecution.RunID, info.ActivityID, info.Attempt)
}

// Transcribe is the activity that transcribes a video
func (ua UtilActivities) Transcribe(
	ctx context.Context,
	input TranscribeParams,
) (*TranscribeResponse, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "Transcribe")
	log.Info("Starting Transcribe")

	if ua.Transcriber == nil {
		return nil, temporal.NewNonRetryableApplicationError("transcriber client not configured", "transcriber_not_configured", nil)
	}

	jobData, err := ua.Transcriber.Transcribe(ctx, input.File.Local(), input.DestinationPath.Local(), input.Language, transcribeIdempotencyKey(ctx))
	if errors.Is(err, transcribe.ErrJobCanceled) {
		// A retry would submit the job again and undo the cancel
		return nil, temporal.NewNonRetryableApplicationError(err.Error(), "transcription_canceled", err)
	}
	if err != nil {
		return nil, err
	}

	log.Info("Finished Transcribe")

	// The old transcriber doesn't list what it wrote, so its file names are assumed.
	if len(jobData.Results) == 0 {
		fileName := input.File.Base()
		return &TranscribeResponse{
			JSONPath:     paths.MustParse(filepath.Join(jobData.OutputPath, fileName+".json")),
			SRTPath:      paths.MustParse(filepath.Join(jobData.OutputPath, fileName+".srt")),
			WordsSRTPath: paths.MustParse(filepath.Join(jobData.OutputPath, fileName+".words.srt")),
			TXTPath:      paths.MustParse(filepath.Join(jobData.OutputPath, fileName+".txt")),
		}, nil
	}

	response, err := transcriptPaths(jobData.Results)
	if err != nil {
		// The same job writes the same files, so a retry would fail the same way.
		return nil, temporal.NewNonRetryableApplicationError(err.Error(), "transcription_results", err)
	}

	return response, nil
}

// transcriptPaths picks each transcript out of the files the transcriber says it
// wrote, so a change to its naming fails here rather than in a later step.
func transcriptPaths(results []string) (*TranscribeResponse, error) {
	byFormat := map[string]string{}
	for _, result := range results {
		// Before the extension check: ".words.srt" also ends in ".srt".
		if strings.HasSuffix(result, ".words.srt") {
			byFormat["words.srt"] = result
			continue
		}
		byFormat[strings.TrimPrefix(filepath.Ext(result), ".")] = result
	}

	var errs []error
	parse := func(format string) paths.Path {
		result, ok := byFormat[format]
		if !ok {
			errs = append(errs, fmt.Errorf("transcriber wrote no %s file", format))
			return paths.Path{}
		}
		path, err := paths.Parse(result)
		if err != nil {
			errs = append(errs, fmt.Errorf("transcriber %s file: %w", format, err))
		}
		return path
	}

	response := &TranscribeResponse{
		JSONPath:     parse("json"),
		SRTPath:      parse("srt"),
		WordsSRTPath: parse("words.srt"),
		TXTPath:      parse("txt"),
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return response, nil
}

type MergeTranscriptJSONParams struct {
	MergeInput      common.MergeInput
	DestinationPath paths.Path
}

type MergeTranscriptResult struct {
	Path paths.Path
}

// MergeTranscriptJSON is the activity that merges a transcript JSON
func (ua UtilActivities) MergeTranscriptJSON(
	ctx context.Context,
	input MergeTranscriptJSONParams,
) (*MergeTranscriptResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "MergeTranscriptJSON")
	log.Info("Starting MergeTranscriptJSON")

	targetFile := input.DestinationPath.Append("merged_transcription.no.json")
	mergedTranscription, err := transcribe.MergeTranscripts(input.MergeInput)
	if err != nil {
		return nil, fmt.Errorf("merging transcripts failed: %w", err)
	}

	marshalled, err := json.Marshal(mergedTranscription)
	if err != nil {
		return nil, err
	}

	_, err = ua.WriteFile(ctx, WriteFileInput{
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
