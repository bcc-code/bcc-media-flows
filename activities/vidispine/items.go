package vsactivity

import (
	"context"
	"go.temporal.io/sdk/activity"
)

type DeleteItemsParams struct {
	VXIDs []string
	//	DeleteFiles bool // TODO: Should probably be implemented in the future
}

// DeleteItems deletes items from Vidispine, including all files on disk!
func (a Activities) DeleteItems(ctx context.Context, params DeleteItemsParams) (any, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting DeleteItems")

	vsClient := GetClient()

	return nil, vsClient.DeleteItems(ctx, params.VXIDs)
}
