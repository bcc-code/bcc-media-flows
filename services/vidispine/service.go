//go:generate mockgen -destination vsmock/mock_VSClient.go -package vsmock . VSClient
package vidispine

import (
	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
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
	CreatePlaceholder(ingestType vsapi.PlaceholderType, title string) (string, error)
	CreateThumbnails(assetID string) error
}

type VidispineService struct {
	apiClient VSClient
}

func NewVidispineService(apiClient VSClient) *VidispineService {
	return &VidispineService{
		apiClient: apiClient,
	}
}

func (s *VidispineService) RegisterFile(filePath string, state vsapi.FileState) (string, error) {
	return s.apiClient.RegisterFile(filePath, state)
}

func (s *VidispineService) AddShapeToItem(shapeTag, itemVXID, fileVXID string) (string, error) {
	return s.apiClient.AddShapeToItem(shapeTag, itemVXID, fileVXID)
}

func (s *VidispineService) AddSidecarToItem(itemVXID, filePath, language string) (string, error) {
	return s.apiClient.AddSidecarToItem(itemVXID, filePath, language)
}

func (s *VidispineService) SetItemMetadataField(itemVXID, field, value string) error {
	return s.apiClient.SetItemMetadataField(itemVXID, field, value)
}

func (s *VidispineService) GetItemMetadataField(vsID string, field vscommon.FieldType) (string, error) {
	meta, err := s.apiClient.GetMetadata(vsID)
	if err != nil {
		return "", err
	}

	return meta.Get(field, ""), nil
}

func (s *VidispineService) GetShapes(itemVXID string) (*vsapi.ShapeResult, error) {
	return s.apiClient.GetShapes(itemVXID)
}

func (s *VidispineService) CreatePlaceholder(ingestType vsapi.PlaceholderType, title string) (string, error) {
	return s.apiClient.CreatePlaceholder(ingestType, title)
}

func (s *VidispineService) CreateThumbnails(assetID string) error {
	return s.apiClient.CreateThumbnails(assetID)
}
