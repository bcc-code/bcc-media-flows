package transcribe

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/utils"

	"github.com/go-resty/resty/v2"
	"go.temporal.io/sdk/activity"
)

const BaseUrl = "http://10.12.128.44:8888"

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

func DebugResponse(resp *resty.Response) {
	fmt.Println("Response Info:")
	fmt.Println("  Status Code:", resp.StatusCode())
	fmt.Println("  Status     :", resp.Status())
	fmt.Println("  Proto      :", resp.Proto())
	fmt.Println("  Time       :", resp.Time())
	fmt.Println("  Received At:", resp.ReceivedAt())
	fmt.Println("  Body       :\n", resp)
	fmt.Println()

	fmt.Println("Request Trace Info:")
	ti := resp.Request.TraceInfo()
	fmt.Println("  DNSLookup     :", ti.DNSLookup)
	fmt.Println("  ConnTime      :", ti.ConnTime)
	fmt.Println("  TCPConnTime   :", ti.TCPConnTime)
	fmt.Println("  TLSHandshake  :", ti.TLSHandshake)
	fmt.Println("  ServerTime    :", ti.ServerTime)
	fmt.Println("  ResponseTime  :", ti.ResponseTime)
	fmt.Println("  TotalTime     :", ti.TotalTime)
	fmt.Println("  IsConnReused  :", ti.IsConnReused)
	fmt.Println("  IsConnWasIdle :", ti.IsConnWasIdle)
	fmt.Println("  ConnIdleTime  :", ti.ConnIdleTime)
	fmt.Println("  RequestAttempt:", ti.RequestAttempt)
	fmt.Println("  RemoteAddr    :", ti.RemoteAddr.String())
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

	restyClient := resty.New()
	restyClient.Debug = true
	restyClient.RetryCount = 3
	restyClient.RetryWaitTime = 10
	restyClient.RetryMaxWaitTime = 30

	language = normalizeTranscriptionLanguage(language)

	resp, err := restyClient.R().EnableTrace().
		SetBody(TranscribeInput{
			Path:       inputFile,
			Language:   language,
			Format:     "all",
			OutputPath: outputFolder,
		}).
		SetResult(&TranscribeJob{}).
		Post(fmt.Sprintf("%s/transcription/job", BaseUrl))

	if err != nil {
		return nil, err
	}

	job := resp.Result().(*TranscribeJob)

	// Periodically check the status of the job
	for {
		activity.RecordHeartbeat(ctx)
		resp, err := restyClient.R().EnableTrace().
			SetResult(&TranscribeJob{}).
			Get(fmt.Sprintf("%s/transcription/job/%s", BaseUrl, job.ID))

		if err != nil {
			return nil, err
		}

		job := resp.Result().(*TranscribeJob)
		switch job.Status {
		case "COMPLETED":
			return job, nil
		case "FAILED":
			return job, fmt.Errorf("job failed")
		}
		time.Sleep(10 * time.Second)
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

func MergeTranscripts(input common.MergeInput) *Transcription {
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
	return mergedTranscription
}
