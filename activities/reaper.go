package activities

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func StartReaper(ctx context.Context) error {
	resp, err := http.Get("http://100.123.200.12:8081/start")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("Received non-200 response status: " + resp.Status)
	}

	return nil
}

type StopReaperResult struct {
	Files []string
}

func StopReaper(ctx context.Context) (*StopReaperResult, error) {
	resp, err := http.Get("http://100.123.200.12:8081/stop")
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
	return &StopReaperResult{
		Files: files,
	}, err
}
