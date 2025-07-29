package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	"os"
)

type GetAudioDiffParams struct {
	ReferenceFile string
	TargetFile    string
}

type GetAudioDiffResult struct {
	Difference int // in milliseconds
}

func (ua UtilActivities) GetAudioDiff(_ context.Context, params GetAudioDiffParams) (*GetAudioDiffResult, error) {
	syncServiceURL := os.Getenv("SYNC_SERVICE_URL")
	client := resty.New()
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]string{
			"reference_file": params.ReferenceFile,
			"target_file":    params.TargetFile,
		}).
		Post(syncServiceURL)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("non-200 response from sync service: %s", resp.String())
	}

	// Parse the JSON response
	var response struct {
		Offset float64 `json:"offset"`
	}

	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert seconds to milliseconds
	differenceMs := int(response.Offset * 1000)

	return &GetAudioDiffResult{
		Difference: differenceMs,
	}, nil
}
