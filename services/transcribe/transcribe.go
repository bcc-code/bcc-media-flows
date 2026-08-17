package transcribe

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"go.temporal.io/sdk/activity"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/internal/httpx"
	"github.com/bcc-code/bcc-media-flows/utils"
)

const BaseUrl = "http://10.12.128.44:8888"

const serviceName = "transcribe"

const (
	retryCount   = 3
	retryWait    = 10 * time.Second
	retryMaxWait = 30 * time.Second
)

// pollInterval and baseURL are vars only so the tests can drive the poll loop against
// a stub server.
var (
	pollInterval = 10 * time.Second
	baseURL      = BaseUrl
)

func newClient() *resty.Client {
	return httpx.New(httpx.Config{
		Service:      serviceName,
		BaseURL:      baseURL,
		RetryCount:   retryCount,
		RetryWait:    retryWait,
		RetryMaxWait: retryMaxWait,
	})
}

var (
	errNoInputFile = fmt.Errorf("no input file")
	errNoOutput    = fmt.Errorf("no output folder")
	errNoLanguage  = fmt.Errorf("no language")
)

type TranscribeInput struct {
	Path       string `json:"path"`
	Language   string `json:"language"`
	Format     string `json:"format"`
	Callback   string `json:"callback,omitempty"`
	OutputPath string `json:"output_path"`
	Priority   int    `json:"priority,omitempty"`
}

type TranscribeJob struct {
	ID           string `json:"id"`
	Path         string `json:"path"`
	Language     string `json:"language"`
	OutputFormat string `json:"format"`
	OutputPath   string `json:"output_path"`
	Progress     int    `json:"progress"`
	Status       string `json:"status"`
	Result       string `json:"result"`
	Callback     string `json:"callback"`
	Model        string `json:"model"`
	Duration     string `json:"duration"`
	Priority     int    `json:"priority"`
}

var whisperSupportedLanguages = map[string]bool{
	"en":  true,
	"zh":  true,
	"de":  true,
	"es":  true,
	"ru":  true,
	"ko":  true,
	"fr":  true,
	"ja":  true,
	"pt":  true,
	"tr":  true,
	"pl":  true,
	"ca":  true,
	"nl":  true,
	"ar":  true,
	"sv":  true,
	"it":  true,
	"id":  true,
	"hi":  true,
	"fi":  true,
	"vi":  true,
	"he":  true,
	"uk":  true,
	"el":  true,
	"ms":  true,
	"cs":  true,
	"ro":  true,
	"da":  true,
	"hu":  true,
	"ta":  true,
	"no":  true,
	"th":  true,
	"ur":  true,
	"hr":  true,
	"bg":  true,
	"lt":  true,
	"la":  true,
	"mi":  true,
	"ml":  true,
	"cy":  true,
	"sk":  true,
	"te":  true,
	"fa":  true,
	"lv":  true,
	"bn":  true,
	"sr":  true,
	"az":  true,
	"sl":  true,
	"kn":  true,
	"et":  true,
	"mk":  true,
	"br":  true,
	"eu":  true,
	"is":  true,
	"hy":  true,
	"ne":  true,
	"mn":  true,
	"bs":  true,
	"kk":  true,
	"sq":  true,
	"sw":  true,
	"gl":  true,
	"mr":  true,
	"pa":  true,
	"si":  true,
	"km":  true,
	"sn":  true,
	"yo":  true,
	"so":  true,
	"af":  true,
	"oc":  true,
	"ka":  true,
	"be":  true,
	"tg":  true,
	"sd":  true,
	"gu":  true,
	"am":  true,
	"yi":  true,
	"lo":  true,
	"uz":  true,
	"fo":  true,
	"ht":  true,
	"ps":  true,
	"tk":  true,
	"nn":  true,
	"mt":  true,
	"sa":  true,
	"lb":  true,
	"my":  true,
	"bo":  true,
	"tl":  true,
	"mg":  true,
	"as":  true,
	"tt":  true,
	"haw": true,
	"ln":  true,
	"ha":  true,
	"ba":  true,
	"jw":  true,
	"su":  true,
	"yue": true,
}

func normalizeTranscriptionLanguage(language string) string {
	language = strings.ToLower(language)

	if language == "auto" || language == "" {
		return language
	}

	if ok, _ := whisperSupportedLanguages[language]; ok {
		return language
	}

	// Try to guess the language
	return "auto"
}

func DoTranscribe(
	ctx context.Context,
	inputFile string,
	outputFolder string,
	language string,
) (*TranscribeJob, error) {

	if inputFile == "" {
		return nil, errNoInputFile
	}

	if outputFolder == "" {
		return nil, errNoOutput
	}

	client := newClient()

	language = normalizeTranscriptionLanguage(language)

	resp, err := client.R().
		SetContext(ctx).
		SetBody(TranscribeInput{
			Path:       inputFile,
			Language:   language,
			Format:     "all",
			OutputPath: outputFolder,
		}).
		SetResult(&TranscribeJob{}).
		Post("/transcription/job")

	if err != nil {
		return nil, err
	}

	job := resp.Result().(*TranscribeJob)
	if job.ID == "" {
		return nil, fmt.Errorf("transcription service accepted the job but returned no id")
	}

	// Periodically check the status of the job
	for {
		// RecordHeartbeat panics outside an activity, and this is a plain function.
		if activity.IsActivity(ctx) {
			activity.RecordHeartbeat(ctx)
		}

		resp, err := client.R().
			SetContext(ctx).
			SetResult(&TranscribeJob{}).
			Get("/transcription/job/" + job.ID)

		if err != nil {
			return nil, err
		}

		status := resp.Result().(*TranscribeJob)
		switch status.Status {
		case "COMPLETED":
			return status, nil
		case "FAILED":
			return status, fmt.Errorf("transcription job %s failed", job.ID)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
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

func MergeTranscripts(input common.MergeInput) (*Transcription, error) {
	mergedTranscription := &Transcription{
		Language: "no",
		Text:     "",
		Segments: []Segment{},
	}

	var errs []error
	startAt := 0.0
	for _, mi := range input.Items {
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

			segment.Start -= mi.Start
			segment.End -= mi.Start

			// Offset the start and end of the segment by duration of the previous cuts
			segment.Start += startAt
			segment.End += startAt

			var words []Word
			for _, word := range segment.Words {
				word.Start -= mi.Start
				word.End -= mi.Start

				word.Start += startAt
				word.End += startAt
				words = append(words, word)
			}

			segment.Words = words

			mergedTranscription.Segments = append(mergedTranscription.Segments, segment)
			mergedTranscription.Text += segment.Text + " "
		}

		startAt += mi.End - mi.Start
	}

	// These were collected and then dropped, so an unreadable input file silently
	// produced a merged transcript missing that whole cut.
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return mergedTranscription, nil
}
