package vidispine

import (
	"context"
	"fmt"
	"os"

	"github.com/bcc-code/bccm-flows/services/vidispine"
	"go.temporal.io/sdk/activity"
)

type GetFileFromVXParams struct {
	VXID string
	Tags []string
}

type GetFileFromVXResult struct {
	FilePath string
	ShapeTag string
}

func GetFileFromVXActivity(ctx context.Context, params GetFileFromVXParams) (*GetFileFromVXResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting GetFileFromVXActivity")

	vsClient := vidispine.NewClient(os.Getenv("VIDISPINE_BASE_URL"), os.Getenv("VIDISPINE_USERNAME"), os.Getenv("VIDISPINE_PASSWORD"))

	shapes, err := vsClient.GetShapes(params.VXID)
	if err != nil {
		return nil, err
	}

	for _, tag := range params.Tags {
		shape := shapes.GetShape(tag)
		if shape == nil {
			log.Debug("No shape found for tag: %s", tag)
			continue
		}

		return &GetFileFromVXResult{
			FilePath: shape.GetPath(),
			ShapeTag: tag,
		}, nil
	}

	return nil, fmt.Errorf("no shape found for tags: %v", params.Tags)
}
