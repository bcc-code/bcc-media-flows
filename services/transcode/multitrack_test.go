package transcode

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"
)

func Test_MultitrackMux(t *testing.T) {
	files := []paths.Path{
		paths.MustParse("./testdata/test_tone_5s.wav"),
		paths.MustParse("./testdata/test_tone_5s.wav"),
		paths.MustParse("./testdata/test_tone_5s.wav"),
		paths.MustParse("./testdata/test_tone_5s.wav"),
	}

	res, err := MultitrackMux(files, files[0].Dir(), nil)

	assert.NoError(t, err)
	assert.NotNil(t, res)
}
