package vidispine

import (
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
)

func GetSubtransID(client VSClient, VXID string) (string, error) {
	meta, err := client.GetMetadata(VXID)
	if err != nil {
		return "", err
	}

	storyId := meta.Get(vscommon.FieldSubtransStoryID, "")
	return storyId, nil
}
