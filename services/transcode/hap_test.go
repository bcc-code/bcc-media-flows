package transcode_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/services/transcode"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
)

func Test_HAP(t *testing.T) {
	testFile := paths.MustParse("./testdata/generated/hap_test_input.mp4")
	outputFile := paths.MustParse("./testdata/generated/results/" + testFile.Base())

	os.MkdirAll(testFile.Dir().Local(), 0755)
	os.MkdirAll(outputFile.Dir().Local(), 0755)

	cmd := exec.Command("ffmpeg",
		"-f", "lavfi",
		"-i", "testsrc=size=1920x1080:rate=25:duration=3",
		"-c:v", "libx264",
		"-y", testFile.Local(),
	)
	err := cmd.Run()
	if err != nil {
		t.Skip("ffmpeg not available for test generation")
	}

	progressCallback := func(i ffmpeg.Progress) {
		spew.Dump(i)
	}

	r, err := transcode.HAP(transcode.HAPInput{
		FilePath:  testFile.Local(),
		OutputDir: outputFile.Dir().Local(),
	}, progressCallback)

	assert.NoError(t, err)
	if !assert.NotNil(t, r) {
		return
	}

	streamInfo, err := ffmpeg.GetStreamInfo(r.OutputPath)
	assert.NoError(t, err)

	assert.True(t, streamInfo.HasVideo)
	assert.Equal(t, 3.0, streamInfo.TotalSeconds)
	assert.Len(t, streamInfo.VideoStreams, 1)

	vs := streamInfo.VideoStreams[0]

	assert.Equal(t, "hap", vs.CodecName)
	assert.Equal(t, 1920, vs.Width)
	assert.Equal(t, 1080, vs.Height)
	assert.Equal(t, "50/1", vs.RFrameRate)

	spew.Dump(r)
}

func Test_HAP_WithAudio(t *testing.T) {
	testFile := paths.MustParse("./testdata/generated/hap_test_audio.mp4")
	outputFile := paths.MustParse("./testdata/generated/results/" + testFile.Base())

	os.MkdirAll(outputFile.Dir().Local(), 0755)

	testutils.GenerateSeparateAudioStreamsTestFile(testFile, 1, 2.0)

	progressCallback := func(i ffmpeg.Progress) {
		spew.Dump(i)
	}

	r, err := transcode.HAP(transcode.HAPInput{
		FilePath:  testFile.Local(),
		OutputDir: outputFile.Dir().Local(),
	}, progressCallback)

	assert.NoError(t, err)
	if !assert.NotNil(t, r) {
		return
	}

	streamInfo, err := ffmpeg.GetStreamInfo(r.OutputPath)
	assert.NoError(t, err)

	assert.True(t, streamInfo.HasVideo)
	assert.True(t, streamInfo.HasAudio)
	assert.Equal(t, 2.0, streamInfo.TotalSeconds)
	assert.Len(t, streamInfo.VideoStreams, 1)

	vs := streamInfo.VideoStreams[0]

	assert.Equal(t, "hap", vs.CodecName)
	assert.Equal(t, 1920, vs.Width)
	assert.Equal(t, 1080, vs.Height)
	assert.Equal(t, "50/1", vs.RFrameRate)

	spew.Dump(vs)
}