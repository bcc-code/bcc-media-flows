package transcode

import (
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_Video(t *testing.T) {
	p, stop := printProgress()
	defer close(stop)

	wm := paths.MustParsePath("/mnt/isilon/system/assets/BTV_LOGO_WATERMARK_BUG_GFX_1080.png")
	_, err := VideoH264(common.VideoInput{
		Path:            paths.MustParsePath("/mnt/isilon/system/tmp/workflows/07c4a523-ee62-481b-b473-80cc82fffb0a/SOTM_7v2123_SEQ.mxf"),
		DestinationPath: paths.MustParsePath("/mnt/isilon/system/tmp/workflows/07c4a523-ee62-481b-b473-80cc82fffb0a"),
		Bitrate:         "1M",
		Width:           1280,
		Height:          720,
		FrameRate:       25,
		WatermarkPath:   &wm,
	}, p)
	assert.Nil(t, err)
}
