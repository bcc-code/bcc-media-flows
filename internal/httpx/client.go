// Package httpx builds resty clients that fail on a non-2xx response, bound every
// attempt with a timeout, and describe their errors the same way.
package httpx

import (
	"context"
	"errors"
	"fmt"
	neturl "net/url"
	"slices"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// DefaultTimeout is generous on purpose: the activity around the call carries the real
// deadline.
const DefaultTimeout = 60 * time.Second

const maxErrorBodyLen = 512

type BasicAuth struct {
	Username string
	Password string
}

type Config struct {
	Service string
	BaseURL string

	// Zero means DefaultTimeout.
	Timeout time.Duration

	// Retries reach every method, POST included, so a retried slow call can duplicate
	// what it creates. Zero leaves resty's defaults for the waits, and no retries.
	RetryCount   int
	RetryWait    time.Duration
	RetryMaxWait time.Duration

	Headers   map[string]string
	BasicAuth *BasicAuth

	// ErrorBody is unmarshalled only for a JSON or XML error body, and reaches
	// DescribeError through resp.Error().
	ErrorBody any

	// Nil means Describe.
	DescribeError func(*resty.Response) error
}

func New(cfg Config) *resty.Client {
	client := resty.New()

	// Several of these services are plain HTTP on the internal network by design.
	client.SetDisableWarn(true)
	client.SetLogger(quietLogger{})

	if cfg.BaseURL != "" {
		client.SetBaseURL(strings.TrimSuffix(cfg.BaseURL, "/"))
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	client.SetTimeout(timeout)

	if cfg.RetryCount > 0 {
		client.SetRetryCount(cfg.RetryCount)
	}
	if cfg.RetryWait > 0 {
		client.SetRetryWaitTime(cfg.RetryWait)
	}
	if cfg.RetryMaxWait > 0 {
		client.SetRetryMaxWaitTime(cfg.RetryMaxWait)
	}

	for name, value := range cfg.Headers {
		client.SetHeader(name, value)
	}

	if cfg.BasicAuth != nil {
		client.SetBasicAuth(cfg.BasicAuth.Username, cfg.BasicAuth.Password)
	}

	if cfg.ErrorBody != nil {
		client.SetError(cfg.ErrorBody)
	}

	describe := cfg.DescribeError
	if describe == nil {
		service := cfg.Service
		describe = func(resp *resty.Response) error { return Describe(service, resp) }
	}

	// IsSuccess rather than IsError: resty calls a status an error only from 400 up,
	// and it unmarshals a result only on 2xx, so a 3xx it did not follow would reach
	// the caller as a zero value with no error.
	client.OnAfterResponse(func(_ *resty.Client, resp *resty.Response) error {
		if resp.IsSuccess() {
			return nil
		}
		if tolerates(resp.Request, resp.StatusCode()) {
			return nil
		}
		return describe(resp)
	})

	return client
}

func Describe(service string, resp *resty.Response) error {
	return DescribeWithDetail(service, resp, TruncateBody(resp.Body()))
}

// DescribeWithDetail replaces the body text with detail, for a DescribeError that has
// read a structured error envelope.
func DescribeWithDetail(service string, resp *resty.Response, detail string) error {
	return &StatusError{
		StatusCode: resp.StatusCode(),
		Message: fmt.Sprintf("%s %s %s failed (status %d): %s",
			service, resp.Request.Method, RedactURL(resp.Request.URL), resp.StatusCode(), detail),
	}
}

// StatusError is what a non-2xx response becomes. Callers that need the status reach
// it with StatusCode(err) rather than by parsing the message.
type StatusError struct {
	StatusCode int
	Message    string
	// Err is an optional sentinel a package wants its callers to match with errors.Is.
	Err error
}

func (e *StatusError) Error() string { return e.Message }

func (e *StatusError) Unwrap() error { return e.Err }

// StatusCode returns the HTTP status err carries, or 0 if it carries none.
func StatusCode(err error) int {
	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode
	}
	return 0
}

var secretQueryParams = []string{"key", "token", "api_key", "apikey", "access_token", "password"}

// SanitizeError covers the errors raised before there is a response to describe: resty
// hands back net/http's *url.Error untouched, and it carries the whole request URL, so
// a DNS failure or a refused connection would leak the query string.
func SanitizeError(err error) error {
	var urlErr *neturl.Error
	if !errors.As(err, &urlErr) {
		return err
	}

	redacted := RedactURL(urlErr.URL)
	if redacted == urlErr.URL {
		return err
	}

	return &neturl.Error{Op: urlErr.Op, URL: redacted, Err: urlErr.Err}
}

// RedactURL keeps an error that names the request from putting a key in the workflow
// history.
func RedactURL(url string) string {
	parsed, err := neturl.Parse(url)
	if err != nil {
		// Whatever made it unparseable may be the credential.
		return "(unparseable url)"
	}

	query := parsed.Query()
	redacted := false
	for _, name := range secretQueryParams {
		for candidate := range query {
			if strings.EqualFold(candidate, name) {
				query.Set(candidate, "redacted")
				redacted = true
			}
		}
	}

	if !redacted {
		return url
	}

	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// quietLogger exists because resty prints the unredacted URL to stdout for every
// failed request.
type quietLogger struct{}

func (quietLogger) Errorf(string, ...any) {}
func (quietLogger) Warnf(string, ...any)  {}
func (quietLogger) Debugf(string, ...any) {}

func TruncateBody(body []byte) string {
	if len(body) <= maxErrorBodyLen {
		return string(body)
	}
	return string(body[:maxErrorBodyLen]) + "…(truncated)"
}

type toleratedKey struct{}

// Tolerating hands the listed statuses back to the caller: an existence probe, or
// deleting something already gone — never a request that should have succeeded.
func Tolerating(req *resty.Request, statuses ...int) *resty.Request {
	return req.SetContext(context.WithValue(req.Context(), toleratedKey{}, statuses))
}

func tolerates(req *resty.Request, status int) bool {
	statuses, _ := req.Context().Value(toleratedKey{}).([]int)
	return slices.Contains(statuses, status)
}
