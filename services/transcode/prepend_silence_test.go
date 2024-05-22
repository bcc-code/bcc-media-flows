package transcode

import (
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/stretchr/testify/assert"
	"os"
	"os/exec"
	"testing"
)

func GenerateTestFile() paths.Path {
	outFile := paths.MustParse("./testdata/generated/test_tone_stereo.wav")
	os.MkdirAll(outFile.Dir().Local(), 0755)

	args := []string{
		"-f", "lavfi",
		"-i", "sine=frequency=300:duration=10:sample_rate=48000",
		"-f", "lavfi",
		"-i", "sine=frequency=1000:duration=10:sample_rate=48000",
		"-filter_complex", "[0:a][1:a]amerge=inputs=2[a]",
		"-map", "[a]",
		"-c:a", "pcm_s16le",
		outFile.Local(),
	}

	cmd := exec.Command("ffmpeg", args...)
	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	return outFile
}

func Test_PrependSilence(t *testing.T) {
	input := GenerateTestFile()
	output := paths.MustParse("./testdata/generated/test_tone_stereo_prefixed.wav")

	cb, _ := printProgress()
	res, err := PrependSilence(input, output, 1.0, 48000, cb)
	assert.NoError(t, err)
	assert.NotEmpty(t, res)

	info, err := ffmpeg.GetStreamInfo(output.Local())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(info.AudioStreams))
	assert.Equal(t, 2, info.AudioStreams[0].Channels)
}
