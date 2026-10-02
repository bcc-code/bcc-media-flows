package cantemo

import (
	"context"
	"fmt"
	"time"

	"github.com/bcc-code/bcc-media-flows/services/cantemo"
	"go.temporal.io/sdk/temporal"
)

type GetFormatsParams struct {
	ItemID string
}

func (a Activities) GetFormats(_ context.Context, params GetFormatsParams) ([]cantemo.Format, error) {
	return a.Client.GetFormats(params.ItemID)
}

type GetItemCreatedParams struct {
	ItemID string
}

// GetItemCreated returns when the item was created in Cantemo.
func (a Activities) GetItemCreated(_ context.Context, params GetItemCreatedParams) (time.Time, error) {
	meta, err := a.Client.GetMetadata(params.ItemID)
	if err != nil {
		return time.Time{}, fmt.Errorf("get metadata for %s: %w", params.ItemID, err)
	}
	if meta.SystemMetadata.Created.IsZero() {
		return time.Time{}, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("item %s has no creation date", params.ItemID), "NO_CREATION_DATE", nil)
	}
	return meta.SystemMetadata.Created, nil
}
