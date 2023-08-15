package transcribe

import (
	"context"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"go.temporal.io/sdk/activity"
)

const BASE_URL = "http://10.12.128.44:8888"

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

	if language == "" {
		return nil, errNoLanguage
	}

	restyClient := resty.New()
	restyClient.Debug = true
	restyClient.RetryCount = 3
	restyClient.RetryWaitTime = 10
	restyClient.RetryMaxWaitTime = 30

	resp, err := restyClient.R().EnableTrace().
		SetBody(TranscribeInput{
			Path:       inputFile,
			Language:   language,
			Format:     "all",
			OutputPath: outputFolder,
		}).
		SetResult(&TranscribeJob{}).
		Post(fmt.Sprintf("%s/transcription/job", BASE_URL))

	if err != nil {
		return nil, err
	}

	job := resp.Result().(*TranscribeJob)

	// Periodically check the status of the job
	for {
		activity.RecordHeartbeat(ctx)
		resp, err := restyClient.R().EnableTrace().
			SetResult(&TranscribeJob{}).
			Get(fmt.Sprintf("%s/transcription/job/%s", BASE_URL, job.ID))

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
