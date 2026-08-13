package transcode

import (
	"github.com/bcc-code/bcc-media-flows/utils"
	"strings"
	"testing"

	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/stretchr/testify/assert"
)

// The golden line is the whole command, assembled the way ffmpeg.Run assembles
// it, so this still covers the ordering that matters — arguments before the
// output, output last.
func Test_GenerateFFmpegParamsForXDCAM(t *testing.T) {
	const golden = `-progress pipe:1 -hide_banner -i something.mxf -c:a copy -c:v mpeg2video -pix_fmt yuv422p -color_primaries bt709 -color_trc bt709 -colorspace bt709 -b:v 50M -s 1920x1080 -r 25 -flags +ilme+ildct -vf setfield=tff,fieldorder=tff -y something/something.mxf`

	input := XDCAMEncodeInput{
		FilePath:   "something.mxf",
		OutputDir:  "out/",
		Resolution: utils.Resolution1080,
		FrameRate:  25,
		Bitrate:    "50M",
		Interlace:  true,
	}

	cmd := ffmpeg.Job{
		Input:  input.FilePath,
		Output: "something/something.mxf",
		Args:   xdcamArgs(input),
	}.Arguments()

	assert.Equal(t, golden, strings.Join(cmd, " "))
}
