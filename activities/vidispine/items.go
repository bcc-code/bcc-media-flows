package vsactivity

import (
	"context"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"go.temporal.io/sdk/activity"
)

type DeleteItemsParams struct {
	VXIDs       []string
	DeleteFiles bool
}

// DeleteItems deletes items from Vidispine, including all files on disk!
func (a Activities) DeleteItems(ctx context.Context, params DeleteItemsParams) (any, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting DeleteItems")

	return nil, a.Client.DeleteItems(ctx, params.VXIDs, params.DeleteFiles)
}

type GetItemsInCollectionParams struct {
	CollectionID string
	Limit        int
}

func (a Activities) GetItemsInCollection(ctx context.Context, params GetItemsInCollectionParams) ([]*vsapi.MetadataResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting GetItemsInCollection")

	res, err := a.Client.GetItemsInCollection(params.CollectionID, params.Limit)
	if err != nil {
		return nil, err
	}

	return res.Items, nil
}
