// Package httpx builds the resty clients the service layer talks to remote APIs with.
//
// A client built here fails on a non-2xx response, bounds every attempt with a
// timeout, and describes its errors the same way every other client does. Those are
// properties of the client rather than something each call site remembers: resty
// leaves err nil for a 500 and only unmarshals a body on 2xx, so a call site that
// forgets the status check reads an outage as an empty result — an empty file
// listing, a zero duration, a delete that deleted nothing.
package httpx

import (
	"context"
	"fmt"
	neturl "net/url"
	"slices"
	"strings"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/go-resty/resty/v2"
)

// DefaultTimeout bounds a single attempt when a Config does not say otherwise.
//
// It is deliberately generous. Every one of these calls runs inside an activity that
// already carries a schedule-to-close budget, so the job here is to stop a hung
// connection from holding a worker slot until that budget expires — not to enforce a
// latency target the remote systems were never asked to meet.
const DefaultTimeout = 60 * time.Second

// maxErrorBodyLen bounds how much of an error body is quoted back. These errors travel
// up into the Temporal workflow history, and an HTML error page from a proxy would
// otherwise put the whole document there.
const maxErrorBodyLen = 512

// BasicAuth carries HTTP basic credentials for a client that needs them.
type BasicAuth struct {
	Username string
	Password string
}

// Config describes one remote API.
type Config struct {
	// Service names the remote system in error messages: "cantemo", "directus".
	Service string

	// BaseURL is prepended to relative request paths. A request made with an absolute
	// URL ignores it, which is what lets a client move over one call at a time.
	BaseURL string

	// Timeout bounds a single attempt. Zero means DefaultTimeout.
	Timeout time.Duration

	// RetryCount is how many times resty retries a failed attempt. Zero means none.
	//
	// Retries apply to every method, including POST, so a client that creates things
	// server-side can duplicate them when a slow call is retried rather than waited
	// out. Prefer a Timeout long enough that a working call finishes.
	RetryCount int

	// Headers are set on every request.
	Headers map[string]string

	// BasicAuth, when set, authenticates every request.
	BasicAuth *BasicAuth

	// ErrorBody is the value resty unmarshals a JSON or XML error body into, reachable
	// from a DescribeError through resp.Error(). Nil leaves error bodies unparsed.
	ErrorBody any

	// DescribeError turns a non-2xx response into the error the caller sees. Nil means
	// Describe. Set it where the service has a structured error envelope worth reading,
	// or a status that carries a sentinel.
	DescribeError func(*resty.Response) error
}

// New builds the client described by cfg.
func New(cfg Config) *resty.Client {
	client := resty.New()

	// resty warns on stdout about auth over a plain connection; several of these
	// services are plain HTTP on the internal network by design.
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

	// Deciding the status here rather than at each call site keeps a new call site safe
	// by default, instead of safe only if its author remembers to check.
	client.OnAfterResponse(func(_ *resty.Client, resp *resty.Response) error {
		if !resp.IsError() {
			return nil
		}
		if tolerates(resp.Request, resp.StatusCode()) {
			return nil
		}
		return describe(resp)
	})

	return client
}

// Describe is the default description of a non-2xx response: which service, which
// request, which status, and as much of the body as is worth carrying.
func Describe(service string, resp *resty.Response) error {
	return DescribeWithDetail(service, resp, TruncateBody(resp.Body()))
}

// DescribeWithDetail is Describe with the body text replaced by detail. Use it from a
// DescribeError that has read a structured error envelope, so the message keeps the
// same shape as every other client's.
func DescribeWithDetail(service string, resp *resty.Response, detail string) error {
	return merry.New(
		fmt.Sprintf("%s %s %s failed (status %d): %s",
			service, resp.Request.Method, RedactURL(resp.Request.URL), resp.StatusCode(), detail),
		merry.WithHTTPCode(resp.StatusCode()),
	)
}

// secretQueryParams are query parameters whose value is a credential. Subtrans
// authenticates with ?key=, and an error message naming the request would otherwise
// carry that key into the Temporal workflow history, where it is readable by anyone
// who can open the execution.
var secretQueryParams = []string{"key", "token", "api_key", "apikey", "access_token", "password"}

// RedactURL returns url with the value of any credential-carrying query parameter
// replaced. A URL it cannot parse is reported as unparseable rather than echoed, since
// the reason it failed to parse may itself be the credential.
func RedactURL(url string) string {
	parsed, err := neturl.Parse(url)
	if err != nil {
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

// quietLogger drops resty's own logging.
//
// resty prints a WARN and an ERROR line to stdout for every failed request, both
// containing the unredacted URL, and both saying what the error this package returns
// already says. The caller logs the error it gets; this stops the credential-bearing
// duplicate.
type quietLogger struct{}

func (quietLogger) Errorf(string, ...any) {}
func (quietLogger) Warnf(string, ...any)  {}
func (quietLogger) Debugf(string, ...any) {}

// TruncateBody renders a response body for an error message, bounded so a proxy's
// HTML error page does not end up in a workflow history in full.
func TruncateBody(body []byte) string {
	if len(body) <= maxErrorBodyLen {
		return string(body)
	}
	return string(body[:maxErrorBodyLen]) + "…(truncated)"
}

// toleratedKey marks a request whose listed statuses are legitimate answers rather
// than failures — an existence probe, or deleting something already gone.
type toleratedKey struct{}

// Tolerating marks a request so the response hook leaves the listed statuses alone and
// hands the response back to the caller. Use it only where the status is an answer the
// caller handles, never to paper over a request that should have succeeded.
func Tolerating(req *resty.Request, statuses ...int) *resty.Request {
	return req.SetContext(context.WithValue(req.Context(), toleratedKey{}, statuses))
}

func tolerates(req *resty.Request, status int) bool {
	statuses, _ := req.Context().Value(toleratedKey{}).([]int)
	return slices.Contains(statuses, status)
}
