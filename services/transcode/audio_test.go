package transcode

import (
	"testing"

	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/paths"
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
