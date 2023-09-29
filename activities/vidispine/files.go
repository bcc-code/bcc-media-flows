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

type CreatePlaceholderParams struct {
	Title string
}

type CreatePlaceholderResult struct {
	AssetID string
}

func CreatePlaceholderActivity(ctx context.Context, params *CreatePlaceholderParams) (*CreatePlaceholderResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting CreatePlaceholderActivity")

	vsClient := GetClient()

	id, err := vsClient.CreatePlaceholder(vsapi.PlaceholderTypeRaw, params.Title)
	if err != nil {
		return nil, err
	}
	return &CreatePlaceholderResult{
		AssetID: id,
	}, nil
}

type CreateThumbnailsParams struct {
	AssetID string
}

func CreateThumbnailsActivity(ctx context.Context, params *CreateThumbnailsParams) error {
	log := activity.GetLogger(ctx)
	log.Info("Starting CreateThumbnailsActivity")

	vsClient := GetClient()

	return vsClient.CreateThumbnails(params.AssetID)
}
