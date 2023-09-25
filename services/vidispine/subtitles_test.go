package vidispine

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_URIParse(t *testing.T) {
	uri := "file:///path/to/file/HC22_20221030_1430_PGM_MU1.mxf"

	parsedUri, err := url.Parse(uri)
	assert.NoError(t, err)
	assert.Equal(t, "/path/to/file/HC22_20221030_1430_PGM_MU1.mxf", parsedUri.Path)
}
