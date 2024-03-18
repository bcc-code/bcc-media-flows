//go:generate mockgen -destination vsmock/mock_VSClient.go -package vsmock . VSClient
package vidispine

import (
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
)

type Client interface {
	GetMetadata(vsID string) (*vsapi.MetadataResult, error)
	GetChapterMeta(itemVXID string, inTc, outTc float64) (map[string]*vsapi.MetadataResult, error)
	GetShapes(itemVXID string) (*vsapi.ShapeResult, error)
	GetSequence(itemVXID string) (*vsapi.SequenceDocument, error)
	RegisterFile(filePath string, state vsapi.FileState) (string, error)
	UpdateFileState(fileID string, fileState vsapi.FileState) error
	AddShapeToItem(shapeTag, itemVXID, fileVXID string) (string, error)
	AddSidecarToItem(itemVXID, filePath, language string) (string, error)
	SetItemMetadataField(itemVXID, group, field, value string) error
	AddToItemMetadataField(itemID, group, key, value string) error
	CreatePlaceholder(ingestType vsapi.PlaceholderType, title string) (string, error)
	CreateThumbnails(assetID string, width, height int) (string, error)
	GetJob(jobID string) (*vsapi.JobDocument, error)
	AddFileToPlaceholder(itemID, fileID, tag string, fileState vsapi.FileState) (string, error)
	GetResolutions(itemVXID string) ([]vsapi.Resolution, error)
}
