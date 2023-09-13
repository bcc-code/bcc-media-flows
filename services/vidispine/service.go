package vidispine

import (
	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
)

type VSClient interface {
	GetMetadata(vsID string) (*vsapi.MetadataResult, error)
	GetChapterMeta(itemVXID string, inTc, outTc float64) (map[string]*vsapi.MetadataResult, error)
	GetShapes(itemVXID string) (*vsapi.ShapeResult, error)
	GetSequence(itemVXID string) (*vsapi.SequenceDocument, error)
}

type VidispineService struct {
	apiClient VSClient
}

func NewVidispineService(apiClient VSClient) *VidispineService {
	return &VidispineService{
		apiClient: apiClient,
	}
}
