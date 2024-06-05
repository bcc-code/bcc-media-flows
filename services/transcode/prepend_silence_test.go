package transcode

import (
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_PrependSilence(t *testing.T) {
	input := paths.MustParse("./testdata/generated/stereo_test.wav")
	testutils.GenerateStreoAudioFile(input, 10)
	output := paths.MustParse("./testdata/generated/test_tone_stereo_prefixed.wav")

	cb, _ := printProgress()
	res, err := PrependSilence(input, output, 1.0, 48000, cb)
	assert.NoError(t, err)
	assert.NotEmpty(t, res)

	info, err := ffmpeg.GetStreamInfo(output.Local())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(info.AudioStreams))
	assert.Equal(t, 2, info.AudioStreams[0].Channels)
}
