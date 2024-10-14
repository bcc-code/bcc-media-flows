package transcode_test

import (
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/services/transcode"
	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_ProRes(t *testing.T) {
	testFile := paths.MustParse("./testdata/generated/prores_1080_1.mov")
	outputFile := paths.MustParse("./testdata/generated/results/" + testFile.Base())

	testutils.GenerateVideoFile(testFile, testutils.VideoGeneratorParams{
		DAR:       "16/9",
		SAR:       "1/1",
		Width:     1920,
		Height:    1080,
		Duration:  5,
		FrameRate: 25,
	})

	progressCallback := func(i ffmpeg.Progress) {
		spew.Dump(i)
	}

	r, err := transcode.ProRes(transcode.ProResInput{
		Resolution: &utils.Resolution{
			Width:  1920,
			Height: 1080,
		},
		FrameRate: 0,
		FilePath:  testFile.Local(),
		OutputDir: outputFile.Dir().Local(),
	}, progressCallback)

	assert.NoError(t, err)
	assert.NotNil(t, r)

	streamInfo, err := ffmpeg.GetStreamInfo(r.OutputPath)
	assert.NoError(t, err)

	assert.True(t, streamInfo.HasVideo)
	assert.Equal(t, 5.0, streamInfo.TotalSeconds)
	assert.Len(t, streamInfo.VideoStreams, 1)

	vs := streamInfo.VideoStreams[0]

	assert.Equal(t, "prores", vs.CodecName)
	assert.Equal(t, "HQ", vs.Profile)
	assert.Equal(t, 1920, vs.Width)
	assert.Equal(t, 1080, vs.Height)
	assert.Equal(t, "1:1", vs.SampleAspectRatio)
	assert.Equal(t, "16:9", vs.DisplayAspectRatio)
	assert.Equal(t, "yuv444p10le", vs.PixFmt)

	spew.Dump(r)
}

func Test_ProResHyperdeck(t *testing.T) {
	testFile := paths.MustParse("./testdata/generated/prores_1080_2.mov")
	outputFile := paths.MustParse("./testdata/generated/results/" + testFile.Base())

	testutils.GenerateVideoFile(testFile, testutils.VideoGeneratorParams{
		DAR:       "16/9",
		SAR:       "1/1",
		Width:     1920,
		Height:    1080,
		Duration:  5,
		FrameRate: 25,
	})

	progressCallback := func(i ffmpeg.Progress) {
		spew.Dump(i)
	}

	r, err := transcode.ProRes(transcode.ProResInput{
		Resolution: &utils.Resolution{
			Width:  1920,
			Height: 1080,
		},
		FrameRate:    0,
		FilePath:     testFile.Local(),
		OutputDir:    outputFile.Dir().Local(),
		ForHyperdeck: true,
	}, progressCallback)

	assert.NoError(t, err)
	assert.NotNil(t, r)

	streamInfo, err := ffmpeg.GetStreamInfo(r.OutputPath)
	assert.NoError(t, err)

	assert.True(t, streamInfo.HasVideo)
	assert.Equal(t, 5.0, streamInfo.TotalSeconds)
	assert.Len(t, streamInfo.VideoStreams, 1)

	vs := streamInfo.VideoStreams[0]

	assert.Equal(t, "prores", vs.CodecName)
	assert.Equal(t, "Standard", vs.Profile)
	assert.Equal(t, 1920, vs.Width)
	assert.Equal(t, 1080, vs.Height)
	assert.Equal(t, "1:1", vs.SampleAspectRatio)
	assert.Equal(t, "16:9", vs.DisplayAspectRatio)
	assert.Equal(t, "yuv422p10le", vs.PixFmt)

	spew.Dump(vs)
}
