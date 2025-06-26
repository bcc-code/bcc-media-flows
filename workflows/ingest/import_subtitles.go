package ingestworkflows

import (
	"encoding/json"
	"fmt"
	"strings"

	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type Transcription struct {
	Text     string    `json:"text"`
	Segments []Segment `json:"segments"`
}

type Segment struct {
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	ID               int     `json:"id"`
	Seek             int     `json:"seek"`
	Tokens           []int   `json:"tokens"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
	Confidence       float64 `json:"confidence"`
	Words            []Word  `json:"words"`
}

type Word struct {
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

type ImportSubtitlesInput struct {
	VXID      string        `json:"vxid"`
	Subtitles Transcription `json:"subtitles"`
	Language  string        `json:"language"`
}

// convertSecondsToSRTTimestamp converts a float64 number of seconds to SRT timestamp format: HH:MM:SS,mmm
func convertSecondsToSRTTimestamp(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	ms := int((seconds - float64(int(seconds))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

// ToSRT generates an SRT string from segments. If words is true, creates a word-level SRT.
func ToSRT(segments []Segment, words bool) string {
	var text strings.Builder
	counter := 1

	for _, segment := range segments {
		if words && len(segment.Words) > 0 {
			for _, word := range segment.Words {
				text.WriteString(fmt.Sprintf("%d\n", counter))
				text.WriteString(convertSecondsToSRTTimestamp(word.Start))
				text.WriteString(" --> ")
				text.WriteString(convertSecondsToSRTTimestamp(word.End))
				text.WriteString("\n")
				text.WriteString(strings.TrimSpace(word.Text))
				text.WriteString("\n\n")
				counter++
			}
			continue
		}
		text.WriteString(fmt.Sprintf("%d\n", counter))
		text.WriteString(convertSecondsToSRTTimestamp(segment.Start))
		text.WriteString(" --> ")
		text.WriteString(convertSecondsToSRTTimestamp(segment.End))
		text.WriteString("\n")
		text.WriteString(strings.TrimSpace(segment.Text))
		text.WriteString("\n\n")
		counter++
	}

	return text.String()
}

// ImportSubtitles imports a subtitle JSON blob for a VX asset, writes it to aux output, and imports it into Vidispine as a shape and as a sidecar
func ImportSubtitles(ctx workflow.Context, input ImportSubtitlesInput) error {
	logger := workflow.GetLogger(ctx)

	if input.VXID == "" {
		return fmt.Errorf("missing VXID")
	}
	if input.Language == "" {
		return fmt.Errorf("missing language")
	}

	outputPath, err := wfutils.GetWorkflowAuxOutputFolder(ctx)
	if err != nil {
		return fmt.Errorf("failed to get aux output folder: %w", err)
	}

	srtFilePath := outputPath.Append(input.VXID + "_subtitles.srt")
	jsonFilePath := outputPath.Append(input.VXID + "_subtitles.json")

	srtData := ToSRT(input.Subtitles.Segments, false)

	err = wfutils.WriteFile(ctx, srtFilePath, []byte(srtData))
	if err != nil {
		return fmt.Errorf("failed to write SRT file: %w", err)
	}

	jsonData, err := json.MarshalIndent(input.Subtitles, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal subtitles to JSON: %w", err)
	}
	err = wfutils.WriteFile(ctx, jsonFilePath, jsonData)
	if err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	logger.Info("Subtitle SRT and JSON written", "srt", srtFilePath.Local(), "json", jsonFilePath.Local())

	// Import SRT as shape
	importSRTJob := wfutils.Execute(ctx, vsactivity.Vidispine.ImportFileAsShapeActivity,
		vsactivity.ImportFileAsShapeParams{
			AssetID:  input.VXID,
			FilePath: srtFilePath,
			ShapeTag: "Transcribed_Subtitle_SRT",
			Replace:  true,
		})

	// Import JSON as shape
	importJSONJob := wfutils.Execute(ctx, vsactivity.Vidispine.ImportFileAsShapeActivity,
		vsactivity.ImportFileAsShapeParams{
			AssetID:  input.VXID,
			FilePath: jsonFilePath,
			ShapeTag: "transcription_json",
			Replace:  true,
		})

	var errs []error
	importSRTResult, err := importSRTJob.Result(ctx)
	if err != nil {
		errs = append(errs, err)
	}

	importJSONResult, err := importJSONJob.Result(ctx)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to import subtitle shapes: %v", errs)
	}

	err = wfutils.WaitForVidispineJob(ctx, importSRTResult.JobID)
	if err != nil {
		return fmt.Errorf("importing of SRT file into Mediabanken failed: %v", err)
	}
	err = wfutils.WaitForVidispineJob(ctx, importJSONResult.JobID)
	if err != nil {
		return fmt.Errorf("importing of JSON file into Mediabanken failed: %v", err)
	}

	// Import SRT as sidecar independently (non-blocking, fire-and-forget)
	err = wfutils.Execute(ctx, vsactivity.Vidispine.ImportFileAsSidecarActivity, vsactivity.ImportSubtitleAsSidecarParams{
		FilePath: srtFilePath,
		Language: input.Language,
		AssetID:  input.VXID,
	}).Wait(ctx)

	if err != nil {
		return fmt.Errorf("importing of SRT file as sidecar failed: %v", err)
	}

	logger.Info("Subtitle SRT and JSON imported as shapes; SRT as sidecar (async)", "vxid", input.VXID)
	return nil
}
