package transcode

import (
	"github.com/bcc-code/bccm-flows/common"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_Audio(t *testing.T) {
	p, stop := printProgress()
	defer close(stop)
	_, err := AudioAac(common.AudioInput{
		Path:            root + "SOTM_7v2123_SEQ-nor.wav",
		DestinationPath: root,
		Bitrate:         "128k",
	}, p)
	assert.Nil(t, err)
}
