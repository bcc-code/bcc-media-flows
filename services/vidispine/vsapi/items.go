package vsapi

import (
	"context"
	"github.com/samber/lo"
	"go.temporal.io/sdk/activity"
	"strings"
)

func (c *Client) DeleteItems(ctx context.Context, id []string, deleteFiles bool) error {
	log := activity.GetLogger(ctx)
	log.Info("Starting DeleteItems")

	batchSize := 20
	chunked := lo.Chunk(id, batchSize)

	url := c.baseURL + "/item"

	for i, chunk := range chunked {
		// Param docs https://apidoc.vidispine.com/5.7/ref/item/item.html#delete-an-item

		log.Info("Deleting chunk %d of %d", i+1, len(chunked))
		// A 404 here means the item is already gone, which is what we wanted. GetTrash
		// and this delete are separate calls, so Cantemo may have purged in between.
		req := tolerating404(c.restyClient.R())
		req.QueryParam.Add("id", strings.Join(chunk, ","))

		if !deleteFiles {
			req.QueryParam.Add("keepShapeTagMedia", "*")
		}

		_, err := req.Delete(url)
		if err != nil {
			return err
		}
	}

	return nil
}
