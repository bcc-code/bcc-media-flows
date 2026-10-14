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
	Model           string
}

type TranscribeResponse struct {
	JSONPath     paths.Path
	SRTPath      paths.Path
	WordsSRTPath paths.Path
	TXTPath      paths.Path
}

// TranscriberJobTimeoutErrorType marks a Transcribe failure because the transcriber
// stopped the job at its time limit. It is not retried
const TranscriberJobTimeoutErrorType = "TRANSCRIBER_JOB_TIMEOUT"

// transcribeIdempotencyKey is per attempt: a reused key returns the old job even
// when it FAILED. Run ID, because reruns can reuse a workflow ID
func transcribeIdempotencyKey(ctx context.Context) string {
	info := activity.GetInfo(ctx)
	return fmt.Sprintf("%s/%s/%d", info.WorkflowExecution.RunID, info.ActivityID, info.Attempt)
}

// TranscribeHeartbeat lets a retry find the job again. Transcriber is only compared,
// never called
type TranscribeHeartbeat struct {
	Version     int    `json:"version"`
	JobID       string `json:"job_id"`
	Transcriber string `json:"transcriber"`
}

const transcribeHeartbeatVersion = 1

// previousTranscribeHeartbeat returns the last attempt's job, or nil for legacy or
// empty details
func previousTranscribeHeartbeat(ctx context.Context) (*TranscribeHeartbeat, error) {
	if !activity.HasHeartbeatDetails(ctx) {
		return nil, nil
	}
	return decodeTranscribeHeartbeat(func(details ...any) error {
		return activity.GetHeartbeatDetails(ctx, details...)
	})
}

func decodeTranscribeHeartbeat(get func(details ...any) error) (*TranscribeHeartbeat, error) {
	var heartbeat TranscribeHeartbeat
	if err := get(&heartbeat); err != nil {
		return nil, err
	}
	if heartbeat.Version < transcribeHeartbeatVersion || heartbeat.JobID == "" || heartbeat.Transcriber == "" {
		return nil, nil
	}
	return &heartbeat, nil
}

// Transcribe is the activity that transcribes a video. Workflows record it by this
// name, so renaming it would break replay of running workflows
func (ta *TranscriberActivities) Transcribe(
	ctx context.Context,
	input TranscribeParams,
) (*TranscribeResponse, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting Transcribe")

	if ta.Client == nil {
		return nil, temporal.NewNonRetryableApplicationError("transcriber client not configured", "transcriber_not_configured", nil)
	}

	previous, err := previousTranscribeHeartbeat(ctx)
	if err != nil {
		log.Warn("Previous attempt's heartbeat names no job, submitting a new one", "error", err)
	}

	return ta.transcribe(ctx, input, previous)
}

// transcribe is split from Transcribe so tests can pass previous directly
func (ta *TranscriberActivities) transcribe(
	ctx context.Context,
	input TranscribeParams,
	previous *TranscribeHeartbeat,
) (*TranscribeResponse, error) {
	log := activity.GetLogger(ctx)

	job, err := ta.resumeOrSubmit(ctx, input, previous)
	if err != nil {
		return nil, err
	}

	// First heartbeat of the attempt, so it isn't throttled. A crash before it lands
	// orphans the job
	heartbeat := TranscribeHeartbeat{Version: transcribeHeartbeatVersion, JobID: job.ID, Transcriber: ta.Client.Identity()}
	activity.RecordHeartbeat(ctx, heartbeat)

	jobData, err := ta.Client.Wait(ctx, job.ID, heartbeat)
	if errors.Is(err, transcribe.ErrJobTimeout) {
		// Another attempt would run into the same limit and spend it on the GPU again
		return nil, temporal.NewNonRetryableApplicationError(err.Error(), TranscriberJobTimeoutErrorType, err)
	}
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

// resumeOrSubmit resumes the previous job if it is on this transcriber and still
// usable, otherwise submits a new one. If the transcriber can't be reached, it submits
// nothing, so no duplicate runs
func (ta *TranscriberActivities) resumeOrSubmit(
	ctx context.Context,
	input TranscribeParams,
	previous *TranscribeHeartbeat,
) (*transcribe.TranscribeJob, error) {
	log := activity.GetLogger(ctx)
	submit := func() (*transcribe.TranscribeJob, error) {
		return ta.Client.Submit(ctx, input.File.Local(), input.DestinationPath.Local(), input.Language, input.Model, transcribeIdempotencyKey(ctx))
	}

	if previous == nil {
		return submit()
	}

	if previous.Transcriber != ta.Client.Identity() {
		log.Warn("Previous attempt's job is on another transcriber, which this worker can't check; submitting a new job",
			"jobID", previous.JobID, "transcriber", previous.Transcriber)
		return submit()
	}

	job, err := ta.Client.Get(ctx, previous.JobID)
	if errors.Is(err, transcribe.ErrJobNotFound) {
		log.Warn("Previous attempt's job is gone from the transcriber, which restarted or dropped it; submitting a new job",
			"jobID", previous.JobID)
		return submit()
	}
	if err != nil {
		return nil, fmt.Errorf("checking previous attempt's transcription job %s: %w", previous.JobID, err)
	}

	switch {
	case job.Status == "FAILED" && !job.TimedOut():
		// What a retry is for. Returning this failure would only have the next attempt
		// find the same job again
		log.Warn("Previous attempt's job failed; submitting a new job", "jobID", job.ID, "error", job.Error)
		return submit()
	case job.Status == "CANCELED":
		// Most likely this activity's own cancel, sent when the last attempt timed out
		// or its worker shut down, so not a reason to give up
		log.Warn("Previous attempt's job was canceled; submitting a new job", "jobID", job.ID)
		return submit()
	}

	// QUEUED and RUNNING are polled on, COMPLETED returns its result, and a timeout
	// fails as one when Wait reads it
	log.Info("Resuming previous attempt's job", "jobID", job.ID, "status", job.Status)
	return job, nil
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
	parse := func(format string, required bool) paths.Path {
		result, ok := byFormat[format]
		if !ok {
			if required {
				errs = append(errs, fmt.Errorf("transcriber wrote no %s file", format))
			}
			return paths.Path{}
		}
		path, err := paths.Parse(result)
		if err != nil {
			errs = append(errs, fmt.Errorf("transcriber %s file: %w", format, err))
		}
		return path
	}

	response := &TranscribeResponse{
		JSONPath:     parse("json", true),
		SRTPath:      parse("srt", true),
		WordsSRTPath: parse("words.srt", false), // no workflow reads it
		TXTPath:      parse("txt", true),
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	// An empty Path can't be decoded from the activity result (its drive is ""), so a
	// missing words.srt gets the name it would have had. Nothing reads it
	if response.WordsSRTPath == (paths.Path{}) {
		response.WordsSRTPath = response.JSONPath.SetExt("words.srt")
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
