package transcode

import (
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_PrependSilence(t *testing.T) {
	input := paths.MustParse("/tmp/test1.wav")
	output := paths.MustParse("/tmp/test1_prefixed.wav")

	cb, _ := printProgress()
	res, err := PrependSilence(input, output, 1.0, 48000, cb)
	assert.NoError(t, err)
	assert.NotEmpty(t, res)
}
