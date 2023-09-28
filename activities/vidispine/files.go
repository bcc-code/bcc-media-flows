package vidispine

import (
	"context"

	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
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

	vsClient := GetClient()

	fileID, err := vsClient.RegisterFile(params.FilePath, vsapi.FileStateClosed)
	if err != nil {
		return err
	}

	_, err = vsClient.AddShapeToItem(params.ShapeTag, params.AssetID, fileID)
	return err
}

type ImportSubtitleAsSidecarParams struct {
	AssetID  string
	FilePath string
	Language string
}

func ImportFileAsSidecarActivity(ctx context.Context, params *ImportSubtitleAsSidecarParams) error {
	log := activity.GetLogger(ctx)
	log.Info("Starting ImportSubtitleAsSidecarParams")

	vsClient := GetClient()

	_, err := vsClient.AddSidecarToItem(params.AssetID, params.FilePath, params.Language)
	return err
}

type ImportFileAsItemParams struct {
	Title    string
	FilePath string
}

func ImportRawMaterialAsItemActivity(ctx context.Context, params *ImportFileAsItemParams) error {
	log := activity.GetLogger(ctx)
	log.Info("Starting ImportRawMaterialAsItemActivity")

	vsClient := GetClient()

	_, err := vsClient.CreatePlaceholder(vsapi.PlaceholderTypeRaw, params.Title)
	return err
}
