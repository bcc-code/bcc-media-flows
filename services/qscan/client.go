// Package qscan is a client for the Quales QScan automated QC REST API.
//
// A QScan job (a "Project" in the GUI) carries an analysis template; files are
// added to a job by repository ID plus a path relative to that repository's
// root, and each file gets an input ID (for status and events) and a results
// ID (for the report).
package qscan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/orsinium-labs/enum"

	"github.com/bcc-code/bcc-media-flows/internal/httpx"
)

const serviceName = "qscan"

var ErrTemplateNotFound = errors.New("qscan template not found")

type Config interface {
	BaseURL() string
	Username() string
	Password() string
}

type Client struct {
	// BaseURL is the GUI address, e.g. "http://10.12.140.18:8080".
	BaseURL string
	client  *resty.Client
}

// apiError covers both envelopes the API uses: {error_code,error_message,url}
// and {status,message}.
type apiError struct {
	ErrorMessage string `json:"error_message"`
	Status       string `json:"status"`
	Message      string `json:"message"`
}

func NewClient(cfg Config) (*Client, error) {
	baseURL := strings.TrimSuffix(cfg.BaseURL(), "/")
	if baseURL == "" {
		return nil, errors.New("qscan base URL not set")
	}

	client := httpx.New(httpx.Config{
		Service:    serviceName,
		BaseURL:    baseURL + "/api-1/qc",
		RetryCount: 3,
		BasicAuth:  &httpx.BasicAuth{Username: cfg.Username(), Password: cfg.Password()},
		ErrorBody:  &apiError{},
		DescribeError: func(resp *resty.Response) error {
			if e, ok := resp.Error().(*apiError); ok {
				detail := e.ErrorMessage
				if detail == "" {
					detail = strings.TrimSpace(e.Status + " " + e.Message)
				}
				if detail != "" {
					return httpx.DescribeWithDetail(serviceName, resp, detail)
				}
			}
			return httpx.Describe(serviceName, resp)
		},
	})

	// Only reads are retried here. A retried POST whose first attempt did reach
	// the server creates a second job or queues the file twice; the activities
	// reconcile against the server instead, and Temporal retries them.
	client.AddRetryCondition(func(resp *resty.Response, err error) bool {
		return err != nil && resp != nil && resp.Request != nil && resp.Request.Method == http.MethodGet
	})

	return &Client{BaseURL: baseURL, client: client}, nil
}

// Int accepts both a JSON number and a quoted number. The API is PHP behind
// the scenes and is not consistent about which one it emits.
type Int int64

func (i *Int) UnmarshalJSON(b []byte) error {
	b = bytes.Trim(bytes.TrimSpace(b), `"`)
	if len(b) == 0 || string(b) == "null" {
		*i = 0
		return nil
	}
	v, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		f, ferr := strconv.ParseFloat(string(b), 64)
		if ferr != nil {
			return fmt.Errorf("qscan: %q is not a number: %w", b, err)
		}
		v = int64(f)
	}
	*i = Int(v)
	return nil
}

type Template struct {
	ID   Int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type CreateJobRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// TemplateID and TemplateName must both be set and refer to the same template.
	TemplateID   int64  `json:"template_id"`
	TemplateName string `json:"template_name"`
	// ConcurrentAnalysis is how many files of the job are analysed at once.
	ConcurrentAnalysis int `json:"concurrent_analysis"`
	// AnalysisPerformance: 1 low, 5 medium, 10 high, 15 very high.
	AnalysisPerformance int `json:"analysis_performance"`
	// ReportSeverity: 0 never, 1 all, 2 warning+critical, 3 critical only.
	ReportSeverity int `json:"report_severity"`
}

type Job struct {
	ID     Int    `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type AddFileRequest struct {
	RepositoryID int64  `json:"repository_id"`
	Path         string `json:"path"`
}

type JobFile struct {
	ID        Int    `json:"id"`
	JobID     Int    `json:"job_id"`
	ResultsID Int    `json:"results_id"`
	Path      string `json:"path"`
}

type Status enum.Member[string]

var (
	StatusDetected       = Status{"detected"}
	StatusReady          = Status{"ready"}
	StatusQueued         = Status{"queued"}
	StatusBlocked        = Status{"blocked"}
	StatusAnalyzing      = Status{"analyzing"}
	StatusPaused         = Status{"paused"}
	StatusAnalyzed       = Status{"analyzed"}
	StatusCanceled       = Status{"canceled"}
	StatusMoved          = Status{"moved"}
	StatusAborted        = Status{"aborted"}
	StatusRemoved        = Status{"removed"}
	StatusReporting      = Status{"reporting"}
	StatusPostAnalysis   = Status{"post_analysis"}
	StatusUnsupported    = Status{"unsupported"}
	StatusEvaluating     = Status{"evaluating"}
	StatusAnalysisError  = Status{"analysis_error"}
	StatusFileError      = Status{"file_error"}
	StatusReanalyze      = Status{"reanalyze"}
	StatusReportingEmail = Status{"reporting_email"}

	Statuses = enum.New(
		StatusDetected, StatusReady, StatusQueued, StatusBlocked, StatusAnalyzing, StatusPaused,
		StatusAnalyzed, StatusCanceled, StatusMoved, StatusAborted, StatusRemoved, StatusReporting,
		StatusPostAnalysis, StatusUnsupported, StatusEvaluating, StatusAnalysisError, StatusFileError,
		StatusReanalyze, StatusReportingEmail,
	)

	terminalStatuses = enum.New(
		StatusAnalyzed, StatusMoved, StatusCanceled, StatusAborted, StatusRemoved,
		StatusUnsupported, StatusAnalysisError, StatusFileError,
	)
)

func (s Status) String() string { return s.Value }

// IsTerminal reports whether the analysis has stopped, for good or bad.
func (s Status) IsTerminal() bool { return terminalStatuses.Contains(s) }

// IsSuccess reports whether the analysis finished and produced a report.
func (s Status) IsSuccess() bool { return s == StatusAnalyzed || s == StatusMoved }

func (s Status) MarshalJSON() ([]byte, error) { return json.Marshal(s.Value) }

func (s *Status) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*s = Status{Value: v}
	return nil
}

type FileStatus struct {
	Status     Status `json:"status"`
	StatusInfo string `json:"status_info"`
	Progress   Int    `json:"progress"`
	// The *_total counts cover the whole analysis; the *_events counts only the
	// header format checks and are a subset.
	CriticalTotal Int `json:"critical_total"`
	WarningTotal  Int `json:"warning_total"`
	LoggingTotal  Int `json:"logging_total"`
}

type Event struct {
	Severity  string `json:"severity"`
	MediaType string `json:"media_type"`
	Message   string `json:"message"`
	TCIn      string `json:"tc_in"`
	TCOut     string `json:"tc_out"`
	Channel   Int    `json:"channel"`
}

type ReportFormat enum.Member[string]

var (
	ReportPDF  = ReportFormat{"pdf"}
	ReportHTML = ReportFormat{"html"}
	ReportJSON = ReportFormat{"json"}
	ReportXML  = ReportFormat{"xml"}
)

// r starts a JSON request. The content type is forced because the server does
// not always label its JSON responses, and resty only decodes labelled ones.
func (c *Client) r(ctx context.Context) *resty.Request {
	return c.client.R().SetContext(ctx).ForceContentType("application/json")
}

func (c *Client) ListTemplates(ctx context.Context) ([]Template, error) {
	var out []Template
	_, err := c.r(ctx).SetResult(&out).Get("/templates")
	return out, err
}

// FindTemplate matches on name, ignoring case and surrounding whitespace.
func (c *Client) FindTemplate(ctx context.Context, name string) (*Template, error) {
	templates, err := c.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	want := strings.ToLower(strings.TrimSpace(name))
	for _, t := range templates {
		if strings.ToLower(strings.TrimSpace(t.Name)) == want {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrTemplateNotFound, name)
}

func (c *Client) ListJobs(ctx context.Context) ([]Job, error) {
	var out []Job
	_, err := c.r(ctx).SetResult(&out).Get("/jobs")
	return out, err
}

// FindJobByName returns the newest job with exactly that name, or nil.
func (c *Client) FindJobByName(ctx context.Context, name string) (*Job, error) {
	jobs, err := c.ListJobs(ctx)
	if err != nil {
		return nil, err
	}
	var found *Job
	for i := range jobs {
		if jobs[i].Name == name && (found == nil || jobs[i].ID > found.ID) {
			found = &jobs[i]
		}
	}
	return found, nil
}

func (c *Client) CreateJob(ctx context.Context, req CreateJobRequest) (*Job, error) {
	var out Job
	_, err := c.r(ctx).SetBody(req).SetResult(&out).Post("/jobs")
	if err != nil {
		return nil, err
	}
	if out.ID == 0 {
		return nil, errors.New("qscan: job created without an id")
	}
	return &out, nil
}

// ResumeJob starts a job that is not running. A 409 means it already is.
func (c *Client) ResumeJob(ctx context.Context, jobID int64) error {
	req := httpx.Tolerating(c.r(ctx), http.StatusConflict)
	_, err := req.Put(fmt.Sprintf("/jobs/%d/resume", jobID))
	return err
}

// decodeFiles accepts the single object the server returns for one file and
// the list it returns when it splits the input.
func decodeFiles(body []byte) ([]JobFile, error) {
	body = bytes.TrimSpace(body)
	if bytes.HasPrefix(body, []byte("[")) {
		var many []JobFile
		if err := json.Unmarshal(body, &many); err != nil {
			return nil, fmt.Errorf("qscan: decoding files: %w", err)
		}
		return many, nil
	}
	var one JobFile
	if err := json.Unmarshal(body, &one); err != nil {
		return nil, fmt.Errorf("qscan: decoding files: %w", err)
	}
	if one.ID == 0 {
		return nil, nil
	}
	return []JobFile{one}, nil
}

// ListJobFiles returns the files queued in a job; a job without files is an
// empty list, not an error.
func (c *Client) ListJobFiles(ctx context.Context, jobID int64) ([]JobFile, error) {
	req := httpx.Tolerating(c.r(ctx), http.StatusNotFound)
	resp, err := req.Get(fmt.Sprintf("/jobs/%d/files", jobID))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, nil
	}
	return decodeFiles(resp.Body())
}

// AddFile queues one file in the job. It is not idempotent on the server, so
// callers check ListJobFiles first.
func (c *Client) AddFile(ctx context.Context, jobID int64, file AddFileRequest) (*JobFile, error) {
	resp, err := c.r(ctx).SetBody([]AddFileRequest{file}).
		Post(fmt.Sprintf("/jobs/%d/files", jobID))
	if err != nil {
		return nil, err
	}
	files, err := decodeFiles(resp.Body())
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("qscan: add file returned no file id: %s", httpx.TruncateBody(resp.Body()))
	}
	return &files[0], nil
}

func (c *Client) FileStatus(ctx context.Context, jobID, fileID int64) (*FileStatus, error) {
	var out FileStatus
	_, err := c.r(ctx).SetResult(&out).
		Get(fmt.Sprintf("/jobs/%d/files/%d/status", jobID, fileID))
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// FileEvents returns every event of the file, flattened and tagged with its
// severity. A file without events is an empty list, not an error.
func (c *Client) FileEvents(ctx context.Context, jobID, fileID int64) ([]Event, error) {
	var groups []struct {
		Severity string  `json:"severity"`
		Events   []Event `json:"events"`
	}
	req := httpx.Tolerating(c.r(ctx).SetResult(&groups), http.StatusNotFound)
	resp, err := req.Get(fmt.Sprintf("/jobs/%d/files/%d/events", jobID, fileID))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, nil
	}
	var events []Event
	for _, g := range groups {
		for _, e := range g.Events {
			e.Severity = g.Severity
			events = append(events, e)
		}
	}
	return events, nil
}

func (c *Client) DownloadReport(ctx context.Context, jobID, resultsID int64, format ReportFormat) ([]byte, error) {
	resp, err := c.client.R().SetContext(ctx).SetQueryParam("type", format.Value).
		Get(fmt.Sprintf("/jobs/%d/files/%d/report", jobID, resultsID))
	if err != nil {
		return nil, err
	}
	if len(resp.Body()) == 0 {
		return nil, errors.New("qscan: empty report")
	}
	return resp.Body(), nil
}
