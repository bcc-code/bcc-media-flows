package transcode

import (
	"github.com/bcc-code/bcc-media-flows/utils"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GenerateFFmpegParamsForXDCAM(t *testing.T) {
	const golden = `-progress pipe:1 -hide_banner -i something.mxf -c:a copy -c:v mpeg2video -pix_fmt yuv422p -color_primaries bt709 -color_trc bt709 -colorspace bt709 -y -b:v 50M -s 1920x1080 -r 25 -flags +ilme+ildct -vf setfield=tff,fieldorder=tff something/something.mxf`

	const outputPath = "something/something.mxf"
	cmd := generateFfmpegParamsForXDCAM(XDCAMEncodeInput{
		FilePath:   "something.mxf",
		OutputDir:  "out/",
		Resolution: utils.Resolution1080,
		FrameRate:  25,
		Bitrate:    "50M",
		Interlace:  true,
	}, outputPath)

	assert.Equal(t, strings.Join(cmd, " "), golden)
}
