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

	"github.com/ansel1/merry/v2"
	"github.com/go-resty/resty/v2"
)

// DefaultTimeout bounds a single attempt when a Config does not say otherwise. It is
// generous on purpose: the activity around the call carries the real deadline.
const DefaultTimeout = 60 * time.Second

const maxErrorBodyLen = 512

type BasicAuth struct {
	Username string
	Password string
}

type Config struct {
	Service string
	BaseURL string

	// Timeout bounds a single attempt. Zero means DefaultTimeout.
	Timeout time.Duration

	// RetryCount applies to every method, POST included, so a retried slow call can
	// duplicate what it creates. Zero means no retries.
	RetryCount int

	// RetryWait and RetryMaxWait bound the backoff. Zero leaves resty's defaults.
	RetryWait    time.Duration
	RetryMaxWait time.Duration

	Headers   map[string]string
	BasicAuth *BasicAuth

	// ErrorBody is what resty unmarshals a JSON or XML error body into, reachable from
	// DescribeError through resp.Error(). Nil leaves error bodies unparsed.
	ErrorBody any

	// DescribeError turns a non-2xx response into the error the caller sees. Nil means
	// Describe.
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

// Describe is the default description of a non-2xx response.
func Describe(service string, resp *resty.Response) error {
	return DescribeWithDetail(service, resp, TruncateBody(resp.Body()))
}

// DescribeWithDetail is Describe with the body text replaced by detail, for a
// DescribeError that has read a structured error envelope.
func DescribeWithDetail(service string, resp *resty.Response, detail string) error {
	return merry.New(
		fmt.Sprintf("%s %s %s failed (status %d): %s",
			service, resp.Request.Method, RedactURL(resp.Request.URL), resp.StatusCode(), detail),
		merry.WithHTTPCode(resp.StatusCode()),
	)
}

var secretQueryParams = []string{"key", "token", "api_key", "apikey", "access_token", "password"}

// SanitizeError redacts a credential from an error raised before there was a response.
// resty hands back net/http's *url.Error untouched, and it carries the whole request
// URL — so a DNS failure or a refused connection reaches the workflow history with the
// query string that Describe would have redacted.
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

// RedactURL replaces the value of any credential-carrying query parameter, so that an
// error naming the request does not carry a key into the workflow history.
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

// quietLogger drops resty's own logging, which prints the unredacted URL to stdout for
// every failed request.
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

// Tolerating hands the listed statuses back to the caller instead of erroring. Use it
// only where the status is an answer the caller handles — an existence probe, or
// deleting something already gone.
func Tolerating(req *resty.Request, statuses ...int) *resty.Request {
	return req.SetContext(context.WithValue(req.Context(), toleratedKey{}, statuses))
}

func tolerates(req *resty.Request, status int) bool {
	statuses, _ := req.Context().Value(toleratedKey{}).([]int)
	return slices.Contains(statuses, status)
}
