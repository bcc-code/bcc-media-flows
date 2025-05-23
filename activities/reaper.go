package activities

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"io"
	"net/http"
)

const reaperBaseUrl = "http://100.123.200.12:8081"

func (l LiveActivities) StartReaper(ctx context.Context, _ any) (string, error) {
	resp, err := http.Get(reaperBaseUrl + "/start")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		telegram.SendText(telegram.ChatOther, fmt.Sprintf("❗❗unable to start reaper. Response: %s\nIngest of video is not impacted.", resp.Status))
	}

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return "", err
	}

	return response["session_id"].(string), nil
}

type ReaperResult struct {
	Files []string
}

func (l LiveActivities) StopReaper(_ context.Context, _ any) (*ReaperResult, error) {
	resp, err := http.Get(reaperBaseUrl + "/stop")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Received non-200 response status: " + resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var files []string
	err = json.Unmarshal(bodyBytes, &files)
	return &ReaperResult{
		Files: files,
	}, err
}

type ListReaperFilesParams struct {
	SessionID string
}

func (l LiveActivities) ListReaperFiles(_ context.Context, params *ListReaperFilesParams) (*ReaperResult, error) {
	resp, err := http.Get(reaperBaseUrl + "/files?session_id=" + params.SessionID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Received non-200 response status: " + resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var files []string
	err = json.Unmarshal(bodyBytes, &files)
	return &ReaperResult{
		Files: files,
	}, err
}
