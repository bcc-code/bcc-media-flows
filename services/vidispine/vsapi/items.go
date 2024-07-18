package vsapi

import (
	"context"
	"github.com/samber/lo"
	"go.temporal.io/sdk/activity"
	"strings"
)

func (c *Client) DeleteItems(ctx context.Context, id []string) error {
	log := activity.GetLogger(ctx)
	log.Info("Starting DeleteItems")

	batchSize := 20
	chunked := lo.Chunk(id, batchSize)

	url := c.baseURL + "/item"

	for i, chunk := range chunked {
		log.Info("Deleting chunk %d of %d", i+1, len(chunked))
		req := c.restyClient.R()
		req.QueryParam.Add("id", strings.Join(chunk, ","))
		_, err := req.Delete(url)
		if err != nil {
			return err
		}
	}

	return nil
}
