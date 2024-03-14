package cantemo

import (
	"context"
	"os"

	"github.com/go-resty/resty/v2"
	"go.temporal.io/sdk/activity"
)

type AddRelationParams struct {
	Parent string
	Child  string
}

func AddRelation(ctx context.Context, params AddRelationParams) (any, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting AddRelationActivity")

	// Warning: this can not have a trailing slash
	urlBase := os.Getenv("CANTEMO_URL")
	token := os.Getenv("CANTEMO_TOKEN")

	client := resty.New()
	client.SetBaseURL(urlBase)
	client.SetHeader("Auth-Token", token)
	client.SetHeader("Accept", "application/json")
	client.SetDisableWarn(true)

	req := client.R()
	res, err := req.Post(urlBase + "/API/v2/items/" + params.Parent + "/relation/" + params.Child + "?type=portal_metadata_cascade&direction=D")

	log.Debug("Response: ", res)

	return nil, err
}
