package transcode_test

import (
	"os"
	"testing"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/services/transcode"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	"github.com/stretchr/testify/assert"
)

func Test_AdjustAudioLevel_Stereo(t *testing.T) {
	testFile := paths.MustParse("./testdata/generated/normalize_stereo.wav")
	outputDir := paths.MustParse("./testdata/generated/results/")
	os.MkdirAll(outputDir.Local(), 0755)

	testutils.GenerateMultichannelAudioFile(testFile, 2, 3)

	res, err := transcode.AdjustAudioLevel(common.AudioInput{
		Path:            testFile,
		DestinationPath: outputDir,
	}, 3.0, func(p ffmpeg.Progress) {})

	assert.NoError(t, err)
	if !assert.NotNil(t, res) {
		return
	}

	info, err := ffmpeg.GetStreamInfo(res.OutputPath.Local())
	assert.NoError(t, err)
	assert.True(t, info.HasAudio)
	assert.Equal(t, 1, len(info.AudioStreams))
	assert.True(t, res.FileSize > 0)
}

func Test_AdjustAudioLevel_Mono(t *testing.T) {
	testFile := paths.MustParse("./testdata/generated/normalize_mono.wav")
	outputDir := paths.MustParse("./testdata/generated/results/")
	os.MkdirAll(outputDir.Local(), 0755)

	testutils.GenerateMultichannelAudioFile(testFile, 1, 3)

	res, err := transcode.AdjustAudioLevel(common.AudioInput{
		Path:            testFile,
		DestinationPath: outputDir,
	}, -6.0, func(p ffmpeg.Progress) {})

	assert.NoError(t, err)
	if !assert.NotNil(t, res) {
		return
	}

	info, err := ffmpeg.GetStreamInfo(res.OutputPath.Local())
	assert.NoError(t, err)
	assert.True(t, info.HasAudio)
	assert.True(t, res.FileSize > 0)
}

func Test_AdjustAudioLevel_TooManyChannels(t *testing.T) {
	testFile := paths.MustParse("./testdata/generated/normalize_4ch.wav")
	outputDir := paths.MustParse("./testdata/generated/results/")
	os.MkdirAll(outputDir.Local(), 0755)

	testutils.GenerateMultichannelAudioFile(testFile, 4, 3)

	_, err := transcode.AdjustAudioLevel(common.AudioInput{
		Path:            testFile,
		DestinationPath: outputDir,
	}, 3.0, func(p ffmpeg.Progress) {})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported for 4 channels")
}
