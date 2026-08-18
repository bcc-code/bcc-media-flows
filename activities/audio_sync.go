package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/environment"

	"github.com/bcc-code/bcc-media-flows/internal/httpx"
)

type GetAudioDiffParams struct {
	ReferenceFile string
	TargetFile    string
}

type GetAudioDiffResult struct {
	Difference int // in milliseconds
}

func (ua UtilActivities) GetAudioDiff(ctx context.Context, params GetAudioDiffParams) (*GetAudioDiffResult, error) {
	syncServiceURL := environment.Get().SyncServiceURL

	client := httpx.New(httpx.Config{
		Service: "audio sync",
		Headers: map[string]string{"Content-Type": "application/json"},
	})

	resp, err := client.R().
		SetContext(ctx).
		SetBody(map[string]string{
			"reference_file": params.ReferenceFile,
			"target_file":    params.TargetFile,
		}).
		Post(syncServiceURL)

	if err != nil {
		return nil, err
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
