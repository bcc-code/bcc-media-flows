package vsactivity

import (
	"context"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"go.temporal.io/sdk/activity"
)

type ImportFileAsShapeParams struct {
	AssetID  string
	FilePath paths.Path
	ShapeTag string
	Growing  bool
}

func ImportFileAsShapeActivity(ctx context.Context, params *ImportFileAsShapeParams) (*JobResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting ImportFileAsShapeActivity")

	vsClient := GetClient()

	fileID, err := vsClient.RegisterFile(params.FilePath.Local(), vsapi.FileStateClosed)
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
	FilePath paths.Path
	Language string
}

type ImportFileAsSidecarResult struct {
	JobID string
}

func ImportFileAsSidecarActivity(ctx context.Context, params *ImportSubtitleAsSidecarParams) (*ImportFileAsSidecarResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting ImportSubtitleAsSidecarParams")

	vsClient := GetClient()

	jobID, err := vsClient.AddSidecarToItem(params.AssetID, params.FilePath.Local(), params.Language)
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

type FileJobResult struct {
	JobID  string
	FileID string
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

type AddFileToPlaceholderParams struct {
	AssetID  string
	FilePath paths.Path
	Tag      string
	Growing  bool
}

func AddFileToPlaceholder(ctx context.Context, params AddFileToPlaceholderParams) (*FileJobResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting AddFileToPlaceholder")

	vsClient := GetClient()

	fileID, err := vsClient.RegisterFile(params.FilePath.Local(), vsapi.FileStateOpen)
	if err != nil {
		return nil, err
	}

	var fileState vsapi.FileState
	if params.Growing {
		fileState = vsapi.FileStateOpen
	} else {
		fileState = vsapi.FileStateClosed
	}

	jobID, err := vsClient.AddFileToPlaceholder(params.AssetID, fileID, params.Tag, fileState)
	if err != nil {
		return nil, err
	}

	return &FileJobResult{
		JobID:  jobID,
		FileID: fileID,
	}, nil
}

type CloseFileParams struct {
	FileID string
}

func CloseFile(ctx context.Context, params CloseFileParams) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting CloseFile")

	vsClient := GetClient()

	return vsClient.UpdateFileState(params.FileID, vsapi.FileStateClosed)
}
