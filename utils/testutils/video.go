package testutils

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/bcc-code/bcc-media-flows/paths"
)

type VideoGeneratorParams struct {
	Duration  float64
	FrameRate int
	Width     int
	Height    int
	SAR       string
	DAR       string
	Profile   string
}

func GenerateVideoFile(outFile paths.Path, videoParams VideoGeneratorParams) paths.Path {
	os.MkdirAll(outFile.Dir().Local(), 0755)
	args := []string{
		"-f", "lavfi",
		"-i", fmt.Sprintf("testsrc=size=%dx%d:rate=%d:duration=%f", videoParams.Width, videoParams.Height, videoParams.FrameRate, videoParams.Duration),
		"-vf", fmt.Sprintf("setsar=%s, setdar=%s", videoParams.SAR, videoParams.DAR),
		"-c:v", "prores_ks",
		"-profile:v", videoParams.Profile,
		"-y", outFile.Local(),
	}

	cmd := exec.Command("ffmpeg", args...)
	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	return outFile
}
