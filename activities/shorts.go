package activities

import (
	"context"
	"fmt"
	"os"

	"github.com/go-resty/resty/v2"
	"go.temporal.io/sdk/activity"
)

var shortServiceURL = os.Getenv("SHORTS_SERVICE_URL")

type SubmitShortJobInput struct {
	InputPath    string    `json:"input_path"`
	OutputPath   string    `json:"output_path"`
	Model        string    `json:"model"`
	Debug        bool      `json:"debug"`
	SceneChanges []float64 `json:"scene_changes"`
}

type Square struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type Keyframe struct {
	EndTimestamp   float64 `json:"end_timestamp"`
	JumpCut        bool    `json:"jump_cut"`
	StartTimestamp float64 `json:"start_timestamp"`
	Square
}

type GenerateShortRequestResult struct {
	Debug     string     `json:"debug"`
	Keyframes []Keyframe `json:"keyframes"`
	Status    string     `json:"status"`
}

type SubmitShortJobResult struct {
	JobID string `json:"job_id"`
}

func (ua UtilActivities) SubmitShortJobActivity(ctx context.Context, params SubmitShortJobInput) (*SubmitShortJobResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "SubmitShortJob")
	log.Info("Starting SubmitShortJob activity")

	restyClient := resty.New()
	var result SubmitShortJobResult
	resp, err := restyClient.R().SetContext(ctx).SetBody(params).SetResult(&result).Post(fmt.Sprintf("%s/submit_job", shortServiceURL))

	if err != nil {
		return nil, fmt.Errorf("resty request failed: %w", err)
	}
	if resp.StatusCode() != 202 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode(), resp.String())
	}

	return &result, nil
}

type CheckJobStatusInput struct {
	JobID string `json:"job_id"`
}

func (ua UtilActivities) CheckJobStatusActivity(ctx context.Context, params CheckJobStatusInput) (*GenerateShortRequestResult, error) {
	activity.RecordHeartbeat(ctx, "CheckJobStatus")

	restyClient := resty.New()

	var result GenerateShortRequestResult
	resp, err := restyClient.R().SetContext(ctx).SetResult(&result).Get(fmt.Sprintf("%s/job_status/%s", shortServiceURL, params.JobID))

	if err != nil {
		return nil, fmt.Errorf("resty request failed: %w", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode(), resp.String())
	}

	return &result, nil
}
