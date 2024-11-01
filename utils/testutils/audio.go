package testutils

import (
	"fmt"
	"github.com/bcc-code/bcc-media-flows/paths"
	"os"
	"os/exec"
)

func GenerateDualMonoAudioFile(outFile paths.Path, length float64) paths.Path {
	os.MkdirAll(outFile.Dir().Local(), 0755)

	args := []string{
		"-f", "lavfi",
		"-i", fmt.Sprintf("sine=frequency=300:duration=%f:sample_rate=48000", length),
		"-f", "lavfi",
		"-i", fmt.Sprintf("sine=frequency=800:duration=%f:sample_rate=48000", length),
		"-map", "0:a",
		"-map", "1:a",
		"-ac", "1",
		"-y", outFile.Local(),
	}

	cmd := exec.Command("ffmpeg", args...)
	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	return outFile
}

func GenerateMultichannelAudioFile(outFile paths.Path, chCount int, length float64) paths.Path {
	os.MkdirAll(outFile.Dir().Local(), 0755)

	args := []string{}

	for i := 0; i < chCount; i++ {
		args = append(args,
			"-f", "lavfi",
			"-i", fmt.Sprintf("sine=frequency=%.f:duration=%f:sample_rate=48000", 100*length, length),
		)
	}

	args = append(args,
		"-filter_complex", fmt.Sprintf("amerge=inputs=%d", chCount),
		"-y", outFile.Local())

	cmd := exec.Command("ffmpeg", args...)
	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	return outFile
}
