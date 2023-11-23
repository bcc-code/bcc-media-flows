//go:generate mockgen -destination vsmock/mock_VSClient.go -package vsmock . VSClient
package vidispine

import (
	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
)

type VSClient interface {
	GetMetadata(vsID string) (*vsapi.MetadataResult, error)
	GetChapterMeta(itemVXID string, inTc, outTc float64) (map[string]*vsapi.MetadataResult, error)
	GetShapes(itemVXID string) (*vsapi.ShapeResult, error)
	GetSequence(itemVXID string) (*vsapi.SequenceDocument, error)
	RegisterFile(filePath string, state vsapi.FileState) (string, error)
	AddShapeToItem(shapeTag, itemVXID, fileVXID string) (string, error)
	AddSidecarToItem(itemVXID, filePath, language string) (string, error)
	SetItemMetadataField(itemVXID, field, value string) error
	AddToItemMetadataField(itemID, key, value string) error
	CreatePlaceholder(ingestType vsapi.PlaceholderType, title string) (string, error)
	CreateThumbnails(assetID string) (string, error)
	GetJob(jobID string) (*vsapi.JobDocument, error)
}
