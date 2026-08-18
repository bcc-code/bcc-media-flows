package vsapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-resty/resty/v2"

	"github.com/bcc-code/bcc-media-flows/internal/httpx"
)

const serviceName = "vidispine"

type Client struct {
	baseURL     string
	username    string
	password    string
	restyClient *resty.Client
}

// tolerating404 is for a probe, or deleting something already gone — never to paper
// over a request that should have succeeded.
func tolerating404(req *resty.Request) *resty.Request {
	return httpx.Tolerating(req, http.StatusNotFound)
}

// vsErrorFromResponse carries ErrShapeTagNotFound so activities/vidispine/files.go can
// detect it with errors.Is and skip pointless retries.
func vsErrorFromResponse(resp *resty.Response) error {
	var envelope vsErrorBody
	if err := json.Unmarshal(resp.Body(), &envelope); err == nil &&
		envelope.NotFound != nil && envelope.NotFound.Type == "shape-tag" {
		return fmt.Errorf("shape-tag %q not configured in Vidispine (%s %s): %w",
			envelope.NotFound.ID, resp.Request.Method, resp.Request.URL, ErrShapeTagNotFound)
	}

	return httpx.Describe(serviceName, resp)
}

type Config interface {
	BaseURL() string
	Username() string
	Password() string
}

func NewClient(cfg Config) *Client {
	baseURL, username, password := cfg.BaseURL(), cfg.Username(), cfg.Password()

	// The retries reach POST and DELETE, so the timeout has to outlast a slow call
	// rather than merely a healthy one: retrying a working AddShapeToItem or
	// CreatePlaceholder creates a second shape or job.
	client := httpx.New(httpx.Config{
		Service:       serviceName,
		BaseURL:       baseURL,
		Headers:       map[string]string{"accept": "application/json"},
		BasicAuth:     &httpx.BasicAuth{Username: username, Password: password},
		RetryCount:    5,
		DescribeError: vsErrorFromResponse,
	})

	return &Client{
		baseURL:     baseURL,
		username:    username,
		password:    password,
		restyClient: client,
	}
}

type IDOnlyResult struct {
	VXID string `json:"id"`
}
