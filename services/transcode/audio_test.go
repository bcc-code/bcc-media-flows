package transcode

import (
	"testing"

	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/paths"
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
