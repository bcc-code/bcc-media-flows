package transcode

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"
)

func Test_MultitrackMux(t *testing.T) {
	files := []paths.Path{
		paths.MustParse("/mnt/temp/test1.wav"),
		paths.MustParse("/mnt/temp/test2.wav"),
		paths.MustParse("/mnt/temp/test3.wav"),
	}
	res, err := MultitrackMux(files, files[0].Dir(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, res)
}
