package transcode_test

import (
	"os"
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/services/transcode"
	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	"github.com/stretchr/testify/assert"
)

func Test_AvcIntra_Progressive(t *testing.T) {
	testFile := paths.MustParse("./testdata/generated/avci_prog.mov")
	outputDir := paths.MustParse("./testdata/generated/results/")
	os.MkdirAll(outputDir.Local(), 0755)

	testutils.GenerateVideoFile(testFile, testutils.VideoGeneratorParams{
		DAR:       "16/9",
		SAR:       "1/1",
		Width:     1920,
		Height:    1080,
		Duration:  3,
		FrameRate: 25,
		Profile:   "3",
	})

	r, err := transcode.AvcIntra(transcode.AVCIntraEncodeInput{
		FilePath:  testFile.Local(),
		OutputDir: outputDir.Local(),
		Resolution: &utils.Resolution{
			Width:  1920,
			Height: 1080,
		},
		FrameRate: 25,
		Interlace: false,
	}, func(p ffmpeg.Progress) {})

	assert.NoError(t, err)
	if !assert.NotNil(t, r) {
		return
	}

	info, err := ffmpeg.GetStreamInfo(r.Path)
	assert.NoError(t, err)
	assert.True(t, info.HasVideo)
	assert.Len(t, info.VideoStreams, 1)

	vs := info.VideoStreams[0]
	assert.Equal(t, "h264", vs.CodecName)
	assert.Equal(t, 1920, vs.Width)
	assert.Equal(t, 1080, vs.Height)
}

func Test_AvcIntra_Interlaced(t *testing.T) {
	testFile := paths.MustParse("./testdata/generated/avci_interlaced.mov")
	outputDir := paths.MustParse("./testdata/generated/results/")
	os.MkdirAll(outputDir.Local(), 0755)

	testutils.GenerateVideoFile(testFile, testutils.VideoGeneratorParams{
		DAR:       "16/9",
		SAR:       "1/1",
		Width:     1920,
		Height:    1080,
		Duration:  3,
		FrameRate: 25,
		Profile:   "3",
	})

	r, err := transcode.AvcIntra(transcode.AVCIntraEncodeInput{
		FilePath:  testFile.Local(),
		OutputDir: outputDir.Local(),
		Resolution: &utils.Resolution{
			Width:  1920,
			Height: 1080,
		},
		FrameRate: 50,
		Interlace: true,
	}, func(p ffmpeg.Progress) {})

	assert.NoError(t, err)
	if !assert.NotNil(t, r) {
		return
	}

	info, err := ffmpeg.GetStreamInfo(r.Path)
	assert.NoError(t, err)
	assert.True(t, info.HasVideo)
	assert.Equal(t, 1920, info.VideoStreams[0].Width)
	assert.Equal(t, 1080, info.VideoStreams[0].Height)
}
