package cantemo

import (
	"context"
	"github.com/bcc-code/bcc-media-flows/services/cantemo"
)

type GetFormatsParams struct {
	ItemID string
}

func (a Activities) GetFormats(_ context.Context, params GetFormatsParams) ([]cantemo.Format, error) {
	return a.Client.GetFormats(params.ItemID)
}
