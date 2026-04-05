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

func Test_MuxToSimpleMXF(t *testing.T) {
	videoFile := paths.MustParse("./testdata/generated/mxf_video.mov")
	audioFile := paths.MustParse("./testdata/generated/mxf_audio.wav")
	outputDir := paths.MustParse("./testdata/generated/results/")
	os.MkdirAll(outputDir.Local(), 0755)

	testutils.GenerateVideoFile(videoFile, testutils.VideoGeneratorParams{
		DAR:       "16/9",
		SAR:       "1/1",
		Width:     1920,
		Height:    1080,
		Duration:  3,
		FrameRate: 25,
		Profile:   "3",
	})

	transcode.GenerateToneFile(1000, 3, 48000, "01:00:00:00", audioFile)

	res, err := transcode.MuxToSimpleMXF(common.SimpleMuxInput{
		FileName:        "test_output",
		VideoFilePath:   videoFile,
		AudioFilePaths:  []paths.Path{audioFile},
		DestinationPath: outputDir,
	}, func(p ffmpeg.Progress) {})

	assert.NoError(t, err)
	if !assert.NotNil(t, res) {
		return
	}

	info, err := ffmpeg.GetStreamInfo(res.Path.Local())
	assert.NoError(t, err)

	assert.True(t, info.HasVideo)
	assert.True(t, info.HasAudio)
	assert.Len(t, info.VideoStreams, 1)
	assert.Len(t, info.AudioStreams, 1)
	assert.Equal(t, "pcm_s24le", info.AudioStreams[0].CodecName)
}

func Test_MuxToSimpleMXF_MultipleAudio(t *testing.T) {
	videoFile := paths.MustParse("./testdata/generated/mxf_video_multi.mov")
	audioFile1 := paths.MustParse("./testdata/generated/mxf_audio1.wav")
	audioFile2 := paths.MustParse("./testdata/generated/mxf_audio2.wav")
	outputDir := paths.MustParse("./testdata/generated/results/")
	os.MkdirAll(outputDir.Local(), 0755)

	testutils.GenerateVideoFile(videoFile, testutils.VideoGeneratorParams{
		DAR:       "16/9",
		SAR:       "1/1",
		Width:     1920,
		Height:    1080,
		Duration:  3,
		FrameRate: 25,
		Profile:   "3",
	})

	transcode.GenerateToneFile(1000, 3, 48000, "01:00:00:00", audioFile1)
	transcode.GenerateToneFile(500, 3, 48000, "01:00:00:00", audioFile2)

	res, err := transcode.MuxToSimpleMXF(common.SimpleMuxInput{
		FileName:        "test_multi_audio",
		VideoFilePath:   videoFile,
		AudioFilePaths:  []paths.Path{audioFile1, audioFile2},
		DestinationPath: outputDir,
	}, func(p ffmpeg.Progress) {})

	assert.NoError(t, err)
	if !assert.NotNil(t, res) {
		return
	}

	info, err := ffmpeg.GetStreamInfo(res.Path.Local())
	assert.NoError(t, err)

	assert.True(t, info.HasVideo)
	assert.True(t, info.HasAudio)
	assert.Len(t, info.VideoStreams, 1)
	assert.Len(t, info.AudioStreams, 2)
}
