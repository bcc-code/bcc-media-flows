package transcode

import (
	"github.com/bcc-code/bccm-flows/common"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_Video(t *testing.T) {
	p, stop := printProgress()
	defer close(stop)
	_, err := VideoH264(common.VideoInput{
		Path:            root + "SOTM_7v2123_SEQ.mxf",
		DestinationPath: root + "../",
		Bitrate:         "2M",
		Width:           1280,
		Height:          720,
		FrameRate:       25,
	}, p)
	assert.Nil(t, err)
}
