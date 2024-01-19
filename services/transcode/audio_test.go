package transcode

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
)

func Test_Audio(t *testing.T) {
	p, stop := printProgress()
	defer close(stop)
	_, err := AudioAac(common.AudioInput{
		Path:            paths.MustParse(root + "SOTM_7v2123_SEQ-nor.wav"),
		DestinationPath: paths.MustParse(root),
		Bitrate:         "128k",
	}, p)
	assert.Nil(t, err)
}

func Test_AudioSplit(t *testing.T) {
	files, err := SplitAudioChannels(paths.MustParse("/tmp/AS23_20231202_2000_PGM_MU1_Joy_to_the_world-eng_normalized-256k.mp3"), paths.MustParse("/tmp/"), nil)

	assert.Nil(t, err)

	spew.Dump(files)
}

func Test_AudioSilence(t *testing.T) {
	isSilent, err := AudioIsSilent(paths.MustParse("/private/temp/workflows/5d2ea767-6b71-44c6-a207-005d7522326c/FKTB_20210415_2000_SEQ-slv.wav"))
	assert.Nil(t, err)

	assert.True(t, isSilent)
}

func Test_AudioChannelSilence(t *testing.T) {
	isSilent, err := AudioStreamIsSilent(paths.MustParse("/private/temp/workflows/5d2ea767-6b71-44c6-a207-005d7522326c/FKTB_20210415_2000_SEQ-slv.wav"), 0, 1, 20)
	assert.Nil(t, err)
	assert.True(t, isSilent)
}
