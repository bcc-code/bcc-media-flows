package activities

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const reaperBaseUrl = "http://100.123.200.12:8081"

func (ua UtilActivities) StartReaper(ctx context.Context, _ any) (any, error) {
	resp, err := http.Get(reaperBaseUrl + "/start")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		return nil, errors.New("Received non-200 response status: " + resp.Status)
	}

	return nil, nil
}

type ReaperResult struct {
	Files []string
}

func (ua UtilActivities) StopReaper(ctx context.Context, _ any) (*ReaperResult, error) {
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

func (ua UtilActivities) ListReaperFiles(ctx context.Context, _ any) (*ReaperResult, error) {
	resp, err := http.Get(reaperBaseUrl + "/files")
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
