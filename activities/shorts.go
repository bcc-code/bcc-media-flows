package activities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.temporal.io/sdk/activity"
)

type SubmitShortJobInput struct {
	URL        string `json:"url"`
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
	Model      string `json:"model"`
	Debug      bool   `json:"debug"`
}

type Keyframe struct {
	EndTimestamp   float64 `json:"end_timestamp"`
	H              int     `json:"h"`
	JumpCut        bool    `json:"jump_cut"`
	StartTimestamp float64 `json:"start_timestamp"`
	W              int     `json:"w"`
	X              int     `json:"x"`
	Y              int     `json:"y"`
}

type GenerateShortRequestResult struct {
	Debug     string     `json:"debug"`
	Keyframes []Keyframe `json:"keyframes"`
	Status    string     `json:"status"`
}

type SubmitShortJobResult struct {
	JobID string `json:"job_id"`
}

func (ua UtilActivities) SubmitShortJob(ctx context.Context, params SubmitShortJobInput) (*SubmitShortJobResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "SubmitShortJob")
	log.Info("Starting SubmitShortJob activity")

	payload := map[string]interface{}{
		"input_path":  params.InputPath,
		"output_path": params.OutputPath,
		"model":       params.Model,
		"debug":       params.Debug,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", params.URL+"/submit_job", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != 202 {
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result SubmitShortJobResult
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

type CheckJobStatusInput struct {
	URL   string `json:"url"`
	JobID string `json:"job_id"`
}

func (ua UtilActivities) CheckJobStatus(ctx context.Context, params CheckJobStatusInput) (*GenerateShortRequestResult, error) {
	// log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "CheckJobStatus")

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/job_status/%s", params.URL, params.JobID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result GenerateShortRequestResult
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}
