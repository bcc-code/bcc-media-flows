//go:build testLive

package vidispine_test

import (
	"testing"

	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/stretchr/testify/assert"
)

func Test_AddFileToPlaceholder(t *testing.T) {
	c := getClient()

	url := c.AddFileToPlaceholder("VX-ITEM", "VX-FILE", "tag", vidispine.FILE_STATE_CLOSED)
	assert.Equal(t, "http://10.12.128.15:8080/import/placeholder/VX-ITEM/container?fileId=VX-FILE&growing=false&tag=tag", url)

	url = c.AddFileToPlaceholder("VX-ITEM", "VX-FILE", "", vidispine.FILE_STATE_CLOSED)
	assert.Equal(t, "http://10.12.128.15:8080/import/placeholder/VX-ITEM/container?fileId=VX-FILE&growing=false", url)

	url = c.AddFileToPlaceholder("VX-ITEM", "VX-FILE", "tag", vidispine.FILE_STATE_OPEN)
	assert.Equal(t, "http://10.12.128.15:8080/import/placeholder/VX-ITEM/container?fastStartLength=7200&fileId=VX-FILE&growing=true&jobmetadata=portal_groups%3AStringArray%253dAdmin&overrideFastStart=true&requireFastStart=true&settings=VX-76&tag=tag", url)

}
