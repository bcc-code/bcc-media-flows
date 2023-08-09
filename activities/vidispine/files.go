package vidispine

import (
	"context"
	"os"

	"github.com/bcc-code/bccm-flows/services/vidispine"
	"go.temporal.io/sdk/activity"
)

type ImportFileAsShapeParams struct {
	AssetID  string
	FilePath string
	ShapeTag string
}

func ImportFileAsShapeActivity(ctx context.Context, params *ImportFileAsShapeParams) error {
	log := activity.GetLogger(ctx)
	log.Info("Starting ImportFileAsShapeActivity")

	vsClient := vidispine.NewClient(os.Getenv("VIDISPINE_BASE_URL"), os.Getenv("VIDISPINE_USERNAME"), os.Getenv("VIDISPINE_PASSWORD"))

	fileID, err := vsClient.RegisterFile(params.FilePath, vidispine.FILE_STATE_CLOSED)
	if err != nil {
		return err
	}

	_, err = vsClient.AddShapeToItem(params.ShapeTag, params.AssetID, fileID)
	return err
}
