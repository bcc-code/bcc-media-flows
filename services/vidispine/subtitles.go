package vidispine

import (
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
)

func (s *VidispineService) GetSubtransID(VXID string) (string, error) {
	meta, err := s.apiClient.GetMetadata(VXID)
	if err != nil {
		return "", err
	}

	storyId := meta.Get(vscommon.FieldSubtransStoryID, "")
	return storyId, nil
}
