package transcode

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GenerateFFmpegParamsForXDCAM(t *testing.T) {
	const golden = `-progress pipe:1 -hide_banner -i something.mxf -c:v mpeg2video -pix_fmt yuv422p -color_primaries bt709 -color_trc bt709 -colorspace bt709 -y -b:v 50M -s 1920x1080 -r 25 -flags +ilme+ildct -top 1 something/something.mxf`

	const root = "root/"
	const outputPath = "something/something.mxf"
	cmd := generateFfmpegParamsForXDCAM(EncodeInput{
		FilePath:   "something.mxf",
		OutputDir:  "out/",
		Resolution: "1920x1080",
		FrameRate:  25,
		Bitrate:    "50M",
		Interlace:  true,
	}, outputPath)

	assert.Equal(t, strings.Join(cmd, " "), golden)
}