package vidispine

import (
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
)

func GetSubtransID(client Client, VXID string) (string, error) {
	meta, err := client.GetMetadata(VXID)
	if err != nil {
		return "", err
	}

	storyId := meta.Get(vscommon.FieldSubtransStoryID, "")
	return storyId, nil
}
