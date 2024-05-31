package testutils

import (
	"github.com/bcc-code/bcc-media-flows/paths"
	"os"
	"os/exec"
)

func GenerateStreoAudioFile(outFile paths.Path, length float64) paths.Path {
	os.MkdirAll(outFile.Dir().Local(), 0755)

	args := []string{
		"-f", "lavfi",
		"-i", "sine=frequency=300:duration=10:sample_rate=48000",
		"-f", "lavfi",
		"-i", "sine=frequency=1000:duration=10:sample_rate=48000",
		"-filter_complex", "[0:a][1:a]amerge=inputs=2[a]",
		"-map", "[a]",
		"-c:a", "pcm_s16le",
		"-y", outFile.Local(),
	}

	cmd := exec.Command("ffmpeg", args...)
	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	return outFile
}
