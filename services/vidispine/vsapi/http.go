package vsapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-resty/resty/v2"

	"github.com/bcc-code/bcc-media-flows/services/internal/httpx"
)

// serviceName names Vidispine in the errors this client returns.
const serviceName = "vidispine"

type Client struct {
	baseURL     string
	username    string
	password    string
	restyClient *resty.Client
}

// tolerating404 marks a request so the response hook leaves its 404 alone. Use it
// only where "not found" is an answer the caller handles, never to paper over a
// request that should have succeeded — a probe, or deleting something already gone.
func tolerating404(req *resty.Request) *resty.Request {
	return httpx.Tolerating(req, http.StatusNotFound)
}

// vsErrorFromResponse turns a non-2xx Vidispine response into an error.
//
// Shape-tag-not-found keeps its ErrShapeTagNotFound sentinel so
// activities/vidispine/files.go can still detect it with errors.Is and skip
// pointless retries, regardless of which request produced it. Everything else is
// described the same way as any other service.
func vsErrorFromResponse(resp *resty.Response) error {
	var envelope vsErrorBody
	if err := json.Unmarshal(resp.Body(), &envelope); err == nil &&
		envelope.NotFound != nil && envelope.NotFound.Type == "shape-tag" {
		return fmt.Errorf("shape-tag %q not configured in Vidispine (%s %s): %w",
			envelope.NotFound.ID, resp.Request.Method, resp.Request.URL, ErrShapeTagNotFound)
	}

	return httpx.Describe(serviceName, resp)
}

func NewClient(baseURL string, username string, password string) *Client {
	// The timeout is deliberately not the 10 seconds this client used to carry.
	// Retries apply to POST and DELETE as well — AddShapeToItem, CreatePlaceholder,
	// DeleteItems — so a call that was merely slow got retried and created a second
	// shape or job. A timeout long enough for a working call to finish is what stops
	// that; the retries stay for the connection failures they were added for.
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
