package transcode

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/stretchr/testify/assert"
)

func Test_SubtitlesBurnIn(t *testing.T) {
	videoPath := paths.Path{
		Drive: paths.TempDrive,
		Path:  "out.mp4",
	}
	subtitlePath := paths.Path{
		Drive: paths.TempDrive,
		Path:  "out.srt",
	}

	outputPath := paths.Path{
		Drive: paths.TempDrive,
		Path:  "",
	}

	p, err := SubtitleBurnIn(videoPath, subtitlePath, outputPath, func(progress ffmpeg.Progress) {
		t.Logf("Progress: %v", progress.Percent)
	})
	assert.Nil(t, err)
	assert.NotNil(t, p)
}
