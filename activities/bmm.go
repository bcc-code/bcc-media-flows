package activities

import (
	"context"
	"fmt"
	"path"

	"go.temporal.io/sdk/activity"

	"github.com/bcc-code/bcc-media-flows/internal/httpx"
)

type TriggerBMMImportInput struct {
	BaseURL      string
	IngestFolder string
}

func (ua UtilActivities) TriggerBMMImport(ctx context.Context, params TriggerBMMImportInput) (any, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TriggerBMMImport")
	log.Info("Starting TriggerBMMImportActivity")

	client := httpx.New(httpx.Config{
		Service: "bmm",
		BaseURL: params.BaseURL,
		Headers: map[string]string{"Content-Type": "application/json"},
	})

	_, err := client.R().
		SetContext(ctx).
		SetQueryParam("path", path.Join(params.IngestFolder, "bmm.json")).
		Post("/events/mediabanken-export/")

	if err != nil {
		return nil, fmt.Errorf("failed to trigger the BMM import: %w", err)
	}

	return nil, nil
}
