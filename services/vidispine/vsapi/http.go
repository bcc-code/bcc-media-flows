package vsapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

type Client struct {
	baseURL     string
	username    string
	password    string
	restyClient *resty.Client
}

// tolerated404Key marks a request whose 404 is a legitimate answer rather than a
// failure — an existence probe, or deleting something that is already gone.
type tolerated404Key struct{}

// tolerating404 marks a request so the response hook leaves its 404 alone. Use it
// only where "not found" is an answer the caller handles, never to paper over a
// request that should have succeeded.
func tolerating404(req *resty.Request) *resty.Request {
	return req.SetContext(context.WithValue(req.Context(), tolerated404Key{}, true))
}

func tolerates404(req *resty.Request) bool {
	tolerated, _ := req.Context().Value(tolerated404Key{}).(bool)
	return tolerated
}

// vsErrorFromResponse turns a non-2xx Vidispine response into an error.
//
// Shape-tag-not-found keeps its ErrShapeTagNotFound sentinel so
// activities/vidispine/files.go can still detect it with errors.Is and skip
// pointless retries, regardless of which request produced it.
func vsErrorFromResponse(resp *resty.Response) error {
	body := resp.Body()
	request := resp.Request.Method + " " + resp.Request.URL

	var envelope vsErrorBody
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("vidispine %s failed (status %d): %s", request, resp.StatusCode(), string(body))
	}

	if envelope.NotFound != nil && envelope.NotFound.Type == "shape-tag" {
		return fmt.Errorf("shape-tag %q not configured in Vidispine (%s): %w",
			envelope.NotFound.ID, request, ErrShapeTagNotFound)
	}

	return fmt.Errorf("vidispine %s failed (status %d): %s", request, resp.StatusCode(), string(body))
}

func NewClient(baseURL string, username string, password string) *Client {
	client := resty.New()
	client.SetBasicAuth(username, password)
	client.SetBaseURL(baseURL)
	client.SetHeader("accept", "application/json")
	client.SetDisableWarn(true)
	client.SetTimeout(10 * time.Second)
	client.SetRetryCount(5)

	// resty leaves err nil for 4xx and 5xx, and because it only unmarshals on 2xx,
	// resp.Result() then hands back a pointer to a zero-valued struct that callers
	// cannot tell apart from a genuinely empty answer. Nearly every request in this
	// package took that path, so a Vidispine outage produced empty shape lists,
	// fallback metadata, and deletes that reported success having done nothing.
	//
	// Converting the status once here rather than at every call site also means a new
	// call site is safe by default instead of safe only if its author remembered.
	client.OnAfterResponse(func(_ *resty.Client, resp *resty.Response) error {
		if !resp.IsError() {
			return nil
		}

		// Probes and idempotent deletes: "not found" is the answer, not a failure.
		if resp.StatusCode() == http.StatusNotFound && tolerates404(resp.Request) {
			return nil
		}

		return vsErrorFromResponse(resp)
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
