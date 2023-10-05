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

func ImportFileAsShapeActivity(ctx context.Context, params *ImportFileAsShapeParams) (*JobResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting ImportFileAsShapeActivity")

	vsClient := GetClient()

	fileID, err := vsClient.RegisterFile(params.FilePath, vsapi.FileStateClosed)
	if err != nil {
		return nil, err
	}

	res, err := vsClient.AddShapeToItem(params.ShapeTag, params.AssetID, fileID)
	return &JobResult{
		JobID: res,
	}, err
}

type ImportSubtitleAsSidecarParams struct {
	AssetID  string
	FilePath string
	Language string
}

type ImportFileAsSidecarResult struct {
	JobID string
}

func ImportFileAsSidecarActivity(ctx context.Context, params *ImportSubtitleAsSidecarParams) (*ImportFileAsSidecarResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting ImportSubtitleAsSidecarParams")

	vsClient := GetClient()

	jobID, err := vsClient.AddSidecarToItem(params.AssetID, params.FilePath, params.Language)
	return &ImportFileAsSidecarResult{
		JobID: jobID,
	}, err
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

type JobResult struct {
	JobID string
}

func CreateThumbnailsActivity(ctx context.Context, params *CreateThumbnailsParams) (*JobResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting CreateThumbnailsActivity")

	vsClient := GetClient()

	res, err := vsClient.CreateThumbnails(params.AssetID)
	return &JobResult{
		JobID: res,
	}, err
}
