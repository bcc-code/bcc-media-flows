package testutils

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bcc-code/bcc-media-flows/paths"
)

func GenerateDualMonoAudioFile(outFile paths.Path, length float64) paths.Path {
	os.MkdirAll(outFile.Dir().Local(), 0755)

	args := []string{
		"-f", "lavfi",
		"-i", fmt.Sprintf("sine=frequency=300:duration=%f:sample_rate=48000:amplitude=0.5+0.5*sin(mul(PI,t)/%f)", length, length),
		"-f", "lavfi",
		"-i", fmt.Sprintf("sine=frequency=800:duration=%f:sample_rate=48000:amplitude=0.5+0.5*sin(mul(PI,t)/%f)", length, length),
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
			"-i", fmt.Sprintf("sine=frequency=%.f:duration=%f:sample_rate=48000:amplitude=0.5+0.5*sin(mul(PI,t)/%f)", 100*float64(i+1), length, length),
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

func GenerateSoftronTestFile(outFile paths.Path, chCount int, length float64) paths.Path {
	os.MkdirAll(outFile.Dir().Local(), 0755)

	args := []string{}
	// Add audio inputs (start at 1, go one longer)
	for i := 1; i <= chCount; i++ {
		args = append(args,
			"-f", "lavfi",
			"-i", fmt.Sprintf("sine=frequency=%d:duration=%f:sample_rate=48000", 100*i, length),
		)
	}

	args = append(args,
		// Generate test video pattern
		"-f", "lavfi",
		"-i", fmt.Sprintf("testsrc=duration=%f:size=1920x1080:rate=50", length),
	)

	args = append(args,
		"-filter_complex", fmt.Sprintf("amerge=inputs=%d", chCount),
		"-c:v", "libx264", // Video codec
		"-c:a", "pcm_s16le", // Audio codec that supports many channels
		"-y", outFile.Local())

	print(strings.Join(args, " "))
	cmd := exec.Command("ffmpeg", args...)
	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	return outFile
}
