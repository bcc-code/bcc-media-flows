package activities

import (
	"context"
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/utils"

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

type MergeTranscriptJSONParams struct {
	MergeInput      common.MergeInput
	DestinationPath paths.Path
}

type MergeTranscriptResult struct {
	Path paths.Path
}

type Transcription struct {
	Text     string    `json:"text"`
	Segments []Segment `json:"segments"`
	Language string    `json:"language"`
}

type Segment struct {
	ID               int     `json:"id"`
	Seek             int     `json:"seek"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	Tokens           []int   `json:"tokens"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
	Words            []Word  `json:"words"`
}

type Word struct {
	Text       string  `json:"text"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Confidence float64 `json:"confidence"`
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

	mergedTranscription := &Transcription{
		Language: "no",
		Text:     "",
		Segments: []Segment{},
	}

	errs := []error{}
	startAt := 0.0
	for _, mi := range input.MergeInput.Items {
		log.Info("Merging", "input", mi.Path.Local())

		transcription := &Transcription{}
		err := utils.JsonFileToStruct(mi.Path.Local(), transcription)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, segment := range transcription.Segments {
			// Ignore segments that are before the start of the cut
			if segment.Start < mi.Start {
				continue
			}

			// Ignore segments that are after the end of the cut
			if segment.Start > mi.End {
				break
			}

			// Offset the start and end of the segment by duration of the previous cuts
			segment.Start += startAt
			segment.End += startAt

			for _, word := range segment.Words {
				word.Start += startAt
				word.End += startAt
			}

			mergedTranscription.Segments = append(mergedTranscription.Segments, segment)
			mergedTranscription.Text += segment.Text + " "
		}

		startAt += mi.End - mi.Start
	}

	marshalled, err := json.Marshal(mergedTranscription)
	if err != nil {
		return nil, err
	}

	WriteFile(ctx, WriteFileInput{
		Path: targetFile,
		Data: marshalled,
	})

	log.Info("Finished MergeTranscriptJSON")

	return &MergeTranscriptResult{
		Path: targetFile,
	}, nil
}
