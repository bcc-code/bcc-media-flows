package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-resty/resty/v2"

	"github.com/bcc-code/bcc-media-flows/internal/httpx"
)

// reaperBaseUrl is where the Reaper recording service lives. A var so the tests can
// point these activities at a stub.
var reaperBaseUrl = "http://100.123.200.12:8081"

func reaperClient() *resty.Client {
	return httpx.New(httpx.Config{
		Service: "reaper",
		BaseURL: reaperBaseUrl,
	})
}

// decodeReaperBody reads a Reaper response body. The bodies are decoded by hand rather
// than through SetResult because /start answers a request to record something already
// recording with 409, and resty unmarshals a result only on 2xx.
func decodeReaperBody(resp *resty.Response, target any) error {
	if err := json.Unmarshal(resp.Body(), target); err != nil {
		return fmt.Errorf("reaper %s returned a body that is not JSON: %w",
			resp.Request.URL, err)
	}
	return nil
}

func (l LiveActivities) StartReaper(ctx context.Context, _ any) (string, error) {
	// 409 means a session is already recording, and its id is the answer to the
	// question this activity asks.
	resp, err := httpx.Tolerating(reaperClient().R().SetContext(ctx), http.StatusConflict).
		Get("/start")
	if err != nil {
		return "", err
	}

	var response struct {
		SessionID string `json:"session_id"`
	}
	if err := decodeReaperBody(resp, &response); err != nil {
		return "", err
	}

	if response.SessionID == "" {
		return "", fmt.Errorf("reaper started no session: no session_id in the response")
	}

	return response.SessionID, nil
}

type ReaperResult struct {
	Files []string
}

func (l LiveActivities) StopReaper(ctx context.Context, _ any) (*ReaperResult, error) {
	resp, err := reaperClient().R().SetContext(ctx).Get("/stop")
	if err != nil {
		return nil, err
	}

	var files []string
	if err := decodeReaperBody(resp, &files); err != nil {
		return nil, err
	}

	return &ReaperResult{Files: files}, nil
}

type ListReaperFilesParams struct {
	SessionID string
}

func (l LiveActivities) ListReaperFiles(ctx context.Context, params *ListReaperFilesParams) (*ReaperResult, error) {
	resp, err := reaperClient().R().
		SetContext(ctx).
		SetQueryParam("session_id", params.SessionID).
		Get("/files")
	if err != nil {
		return nil, err
	}

	var files []string
	if err := decodeReaperBody(resp, &files); err != nil {
		return nil, err
	}

	return &ReaperResult{Files: files}, nil
}
