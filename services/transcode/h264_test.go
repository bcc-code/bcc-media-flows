package transcode_test

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/services/transcode"
	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
)

func Test_H264_WeirdResolutions(t *testing.T) {
	testFile := paths.MustParse("./testdata/generated/h264_weird_resolutions.mov")

	testutils.GenerateVideoFile(testFile, testutils.VideoGeneratorParams{
		DAR:       "16/9",
		SAR:       "608/405",
		Width:     720,
		Height:    608,
		Duration:  5,
		FrameRate: 25,
		Profile:   "2",
	})

	progressCallback := func(i ffmpeg.Progress) {
		spew.Dump(i)
	}

	r, err := transcode.H264(transcode.H264EncodeInput{
		Bitrate: "320k",
		Resolution: &utils.Resolution{
			Width:  320,
			Height: 180,
		},
		FrameRate: 0,
		FilePath:  testFile.Local(),
		OutputDir: testFile.Dir().Local(),
	}, progressCallback)

	assert.NoError(t, err)
	assert.NotNil(t, r)
	spew.Dump(r)
}
