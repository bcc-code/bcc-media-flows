package transcode

import (
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	"os"
	"testing"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/stretchr/testify/assert"
)

func Test_Audio(t *testing.T) {
	tempDstPath := paths.MustParse("./testdata/test" + t.Name() + ".wav")
	err := GenerateToneFile(1000, 5, 48000, "01:00:00:00", tempDstPath)
	assert.NoError(t, err)

	p, stop := printProgress()
	defer close(stop)
	res, err := AudioAac(common.AudioInput{
		Path:            tempDstPath,
		DestinationPath: tempDstPath.Dir(),
		Bitrate:         "128k",
	}, p)
	assert.Nil(t, err)
	assert.NotNil(t, res)

	info, err := ffmpeg.GetStreamInfo(res.OutputPath.Local())
	assert.NoError(t, err)

	assert.True(t, info.HasAudio)
	assert.False(t, info.HasVideo)
	assert.InDelta(t, 5, 0.2, info.TotalSeconds)
	assert.Equal(t, 1, len(info.AudioStreams))
}

func Test_AudioSplit_Stereo(t *testing.T) {
	tempDstPath := paths.MustParse("./testdata/test" + t.Name() + ".wav")
	testutils.GenerateStreoAudioFile(tempDstPath, 10)

	p, stop := printProgress()
	defer close(stop)
	files, err := SplitAudioChannels(tempDstPath, tempDstPath.Dir(), p)

	assert.Nil(t, err)
	assert.Len(t, files, 2)
}

func Test_AudioSilence(t *testing.T) {
	isSilent, err := AudioIsSilent(paths.MustParse("/private/temp/workflows/5d2ea767-6b71-44c6-a207-005d7522326c/FKTB_20210415_2000_SEQ-slv.wav"))
	assert.Nil(t, err)

	assert.True(t, isSilent)
}

func Test_AudioChannelSilence(t *testing.T) {
	isSilent, err := AudioStreamIsSilent(paths.MustParse("/private/temp/workflows/5d2ea767-6b71-44c6-a207-005d7522326c/FKTB_20210415_2000_SEQ-slv.wav"), 0, 1, 20)
	assert.Nil(t, err)
	assert.True(t, isSilent)
}

func Test_ToneGenerator(t *testing.T) {
	tempDstPath := paths.MustParse("./testdata/test.wav")
	err := GenerateToneFile(1000, 5, 48000, "01:00:00:00", tempDstPath)
	assert.Nil(t, err)
	assert.FileExistsf(t, tempDstPath.Local(), "File should exist")
	fileCanBeDeleted := true
	_, err = os.Stat(tempDstPath.Local())
	fileCanBeDeleted = err == nil
	assert.NoError(t, err)

	probe, err := ffmpeg.ProbeFile(tempDstPath.Local())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(probe.Streams))
	assert.Equal(t, "5.000000", probe.Format.Duration)
	assert.Equal(t, "audio", probe.Streams[0].CodecType)
	assert.Equal(t, "1/48000", probe.Streams[0].TimeBase)
	assert.Equal(t, "24", probe.Streams[0].BitsPerRawSample)
	assert.Equal(t, 1, probe.Streams[0].Channels)

	tc, err := ffmpeg.GetTimeReference(tempDstPath.Local())
	assert.NoError(t, err)
	assert.Equal(t, 172800000, tc)

	if fileCanBeDeleted {
		err = os.Remove(tempDstPath.Local())
		assert.NoError(t, err)
	}
}
