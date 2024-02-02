package vsactivity

import (
	"context"
	"fmt"

	"github.com/bcc-code/bcc-media-flows/paths"
	"go.temporal.io/sdk/activity"
)

type GetFileFromVXParams struct {
	VXID string
	Tags []string
}

type GetFileFromVXResult struct {
	FilePath paths.Path
	ShapeTag string
}

func GetFileFromVXActivity(ctx context.Context, params GetFileFromVXParams) (*GetFileFromVXResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting GetFileFromVXActivity")

	vsClient := GetClient()

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
			FilePath: paths.MustParse(shape.GetPath()),
			ShapeTag: tag,
		}, nil
	}

	return nil, fmt.Errorf("no shape found for tags: %v", params.Tags)
}

type SetVXMetadataFieldParams struct {
	VXID  string
	Key   string
	Value string
	Group string
}

type SetVXMetadataFieldResult struct {
}

func SetVXMetadataFieldActivity(ctx context.Context, params SetVXMetadataFieldParams) (*SetVXMetadataFieldResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting SetVXMetadataFieldActivity")

	vsClient := GetClient()

	err := vsClient.SetItemMetadataField(params.VXID, params.Group, params.Key, params.Value)
	return nil, err
}

func AddVXMetadataFieldValueActivity(ctx context.Context, params SetVXMetadataFieldParams) (*SetVXMetadataFieldResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting AddVXMetadataFieldValueActivity")

	vsClient := GetClient()

	err := vsClient.AddToItemMetadataField(params.VXID, params.Group, params.Key, params.Value)
	return nil, err
}
