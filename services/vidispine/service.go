//go:generate mockgen -destination vsmock/mock_Client.go -package vsmock . Client
package vidispine

import (
	"context"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
)

type Client interface {
	GetMetadata(vsID string) (*vsapi.MetadataResult, error)
	// GetChapterMeta returns all "Subclips" for a given itemVXID, in the given timecode range.
	//
	// The timecodes are in seconds.
	//
	// The result is a map with the key being the clip key (See SplitByClips) and the value being the metadata result.
	GetChapterMeta(itemVXID string, inTc, outTc float64) (map[string]*vsapi.MetadataResult, error)
	GetShapes(itemVXID string) (*vsapi.ShapeResult, error)
	GetSequence(itemVXID string) (*vsapi.SequenceDocument, error)
	RegisterFile(filePath string, state vsapi.FileState) (string, error)
	UpdateFileState(fileID string, fileState vsapi.FileState) error
	AddShapeToItem(shapeTag, itemVXID, fileVXID string) (string, error)
	DeleteShape(assetID, shapeID string) error
	AddSidecarToItem(itemVXID, filePath, language string) (string, error)
	SetItemMetadataField(params vsapi.ItemMetadataFieldParams) error
	AddToItemMetadataField(params vsapi.ItemMetadataFieldParams) error
	CreatePlaceholder(ingestType vsapi.PlaceholderType, title string) (string, error)
	CreateThumbnails(assetID string, width, height int) (string, error)
	GetJob(jobID string) (*vsapi.JobDocument, error)
	FindJob(itemID string, jobType string) (*vsapi.JobDocument, error)
	AddFileToPlaceholder(itemID, fileID, tag string, fileState vsapi.FileState) (string, error)
	GetResolutions(itemVXID string) ([]vsapi.Resolution, error)
	GetRelations(assetID string) ([]vsapi.Relation, error)
	GetTrash() ([]string, error)
	DeleteItems(ctx context.Context, itemVXIDs []string, deleteFiles bool) error
}
