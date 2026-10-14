package transcribe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"go.temporal.io/sdk/activity"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/internal/httpx"
	"github.com/bcc-code/bcc-media-flows/utils"
)

const serviceName = "transcribe"

const (
	retryCount   = 3
	retryWait    = 10 * time.Second
	retryMaxWait = 30 * time.Second

	cancelTimeout = 30 * time.Second
)

// pollInterval is a var only so the tests can drive the poll loop against a stub
// server.
var pollInterval = 10 * time.Second

// ErrJobCanceled means the job was canceled on the transcriber itself, which only
// happens when someone stops it there on purpose
var ErrJobCanceled = errors.New("transcription job canceled")

// ErrJobTimeout means the transcriber stopped the job at its time limit (JOB_TIMEOUT,
// or the job's own timeout_seconds)
var ErrJobTimeout = errors.New("stopped at the transcriber's job timeout")

// jobTimeoutError is the whole error value the transcriber documents for a job it
// stopped at its time limit. It has no error code, so this exact value is the signal;
// any other text, even one mentioning a timeout, is an ordinary failure
const jobTimeoutError = "timeout"

// ErrJobNotFound means the transcriber doesn't know the job: it restarted, which also
// kills running jobs, or it dropped the finished job from its store
var ErrJobNotFound = errors.New("transcriber has no such job")

var (
	errNoBaseURL   = errors.New("transcriber base URL not set")
	errNoInputFile = errors.New("no input file")
	errNoOutput    = errors.New("no output folder")
)

type Config interface {
	Transcriber() string
}

// Client talks to one transcriber instance. Job IDs are only known to the instance
// that issued them, so a job is submitted and polled through the same client.
type Client struct {
	client   *resty.Client
	identity string
}

// Identity is the transcriber's base URL without credentials. Unique only if each host
// uses its own address, not a shared alias
func (c *Client) Identity() string { return c.identity }

func NewClient(cfg Config) (*Client, error) {
	baseURL := cfg.Transcriber()
	if baseURL == "" {
		return nil, errNoBaseURL
	}

	return &Client{
		identity: transcriberIdentity(baseURL),
		client: httpx.New(httpx.Config{
			Service:      serviceName,
			BaseURL:      baseURL,
			RetryCount:   retryCount,
			RetryWait:    retryWait,
			RetryMaxWait: retryMaxWait,
		}),
	}, nil
}

type TranscribeInput struct {
	Path       string `json:"path"`
	Language   string `json:"language"`
	Format     string `json:"format"`
	Callback   string `json:"callback,omitempty"`
	OutputPath string `json:"output_path"`
	Priority   int    `json:"priority,omitempty"`
	Model      string `json:"model,omitempty"`
}

type TranscribeJob struct {
	ID           string   `json:"id"`
	Path         string   `json:"path"`
	Language     string   `json:"language"`
	OutputFormat string   `json:"format"`
	OutputPath   string   `json:"output_path"`
	Progress     int      `json:"progress"`
	Status       string   `json:"status"`
	Result       string   `json:"result"`
	Results      []string `json:"results,omitempty"`
	Callback     string   `json:"callback"`
	Model        string   `json:"model"`
	Duration     string   `json:"duration"`
	Priority     int      `json:"priority"`
	Error        string   `json:"error,omitempty"`
}

// TimedOut reports whether the transcriber stopped the job at its time limit
func (j *TranscribeJob) TimedOut() bool {
	return j.Status == "FAILED" && j.Error == jobTimeoutError
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

// transcriberIdentity normalises a base URL and drops credentials. An unparseable URL
// gives "", which turns recovery off
func transcriberIdentity(baseURL string) string {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u.Host == "" {
		return ""
	}
	u.User = nil
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimSuffix(u.String(), "/")
}

// Transcribe submits a job and waits for it to end
func (c *Client) Transcribe(
	ctx context.Context,
	inputFile string,
	outputFolder string,
	language string,
	model string,
	idempotencyKey string,
) (*TranscribeJob, error) {
	job, err := c.Submit(ctx, inputFile, outputFolder, language, model, idempotencyKey)
	if err != nil {
		return nil, err
	}

	return c.Wait(ctx, job.ID, nil)
}

// Submit hands a new job to the transcriber and returns it as accepted
func (c *Client) Submit(
	ctx context.Context,
	inputFile string,
	outputFolder string,
	language string,
	model string,
	idempotencyKey string,
) (*TranscribeJob, error) {
	if inputFile == "" {
		return nil, errNoInputFile
	}

	if outputFolder == "" {
		return nil, errNoOutput
	}

	language = normalizeTranscriptionLanguage(language)

	req := c.client.R().
		SetContext(ctx).
		SetBody(TranscribeInput{
			Path:       inputFile,
			Language:   language,
			Format:     "all",
			OutputPath: outputFolder,
			Model:      model,
		}).
		SetResult(&TranscribeJob{})

	// The client retries the POST too, and a retry of one the server did accept would
	// otherwise start a second job
	if idempotencyKey != "" {
		req.SetHeader("Idempotency-Key", idempotencyKey)
	}

	resp, err := req.Post("/transcription/job")
	if err != nil {
		return nil, err
	}

	job := resp.Result().(*TranscribeJob)
	if job.ID == "" {
		return nil, errors.New("transcription service accepted the job but returned no id")
	}

	return job, nil
}

// Get reads a job's current state once
func (c *Client) Get(ctx context.Context, jobID string) (*TranscribeJob, error) {
	resp, err := c.client.R().
		SetContext(ctx).
		SetResult(&TranscribeJob{}).
		Get("/transcription/job/" + jobID)
	if httpx.StatusCode(err) == http.StatusNotFound {
		return nil, fmt.Errorf("transcription job %s: %w: %w", jobID, ErrJobNotFound, err)
	}
	if err != nil {
		return nil, err
	}

	return resp.Result().(*TranscribeJob), nil
}

// Wait polls a job until it ends. heartbeat, when not nil, is recorded on every poll,
// so a retry can find the job again
func (c *Client) Wait(ctx context.Context, jobID string, heartbeat any) (_ *TranscribeJob, err error) {
	// When the activity is cancelled or times out, the job would otherwise keep a GPU
	// busy for nobody, and a retry would start a second one beside it
	defer func() {
		if err != nil && ctx.Err() != nil {
			err = errors.Join(err, c.cancelJob(ctx, jobID))
		}
	}()

	for {
		// RecordHeartbeat panics outside an activity, and this is a plain function.
		if activity.IsActivity(ctx) {
			if heartbeat != nil {
				activity.RecordHeartbeat(ctx, heartbeat)
			} else {
				activity.RecordHeartbeat(ctx)
			}
		}

		job, err := c.Get(ctx, jobID)
		if err != nil {
			return nil, err
		}

		switch job.Status {
		case "COMPLETED":
			return job, nil
		case "FAILED":
			if job.TimedOut() {
				return job, fmt.Errorf("transcription job %s failed: %w", jobID, ErrJobTimeout)
			}
			if job.Error != "" {
				return job, fmt.Errorf("transcription job %s failed: %s", jobID, job.Error)
			}
			return job, fmt.Errorf("transcription job %s failed", jobID)
		case "CANCELED":
			return job, fmt.Errorf("transcription job %s: %w", jobID, ErrJobCanceled)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// cancelJob runs because ctx is already done, so it gets a short context of its own
func (c *Client) cancelJob(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cancelTimeout)
	defer cancel()

	// 404: the job has already been dropped from the transcriber's store
	_, err := httpx.Tolerating(c.client.R().SetContext(ctx), http.StatusNotFound).
		Delete("/transcription/job/" + id)
	if err != nil {
		return fmt.Errorf("cancel transcription job %s: %w", id, err)
	}

	return nil
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
