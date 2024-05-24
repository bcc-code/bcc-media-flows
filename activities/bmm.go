package activities

import (
	"context"
	"fmt"
	"go.temporal.io/sdk/activity"
	"net/http"
	"net/url"
	"path"
)

type TriggerBMMImportInput struct {
	BaseURL      string
	IngestFolder string
}

func (ua UtilActivities) TriggerBMMImport(ctx context.Context, params TriggerBMMImportInput) (any, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TriggerBMMImport")
	log.Info("Starting TriggerBMMImportActivity")

	trigger := params.BaseURL + "/events/mediabanken-export/?path="
	jsonS3Path := path.Join(params.IngestFolder, "bmm.json")
	trigger += url.QueryEscape(jsonS3Path)

	resp, err := http.Post(trigger, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to BMM: %w", err)
	}

	resp.Body.Close()

	if resp.StatusCode > 200 {
		return nil, fmt.Errorf("BMM returned unexpected status code: %s", resp.Status)
	}

	return nil, nil
}
