package activities

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

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
