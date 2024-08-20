package testutils

import (
	"fmt"
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
}

func GenerateVideoFile(outFile paths.Path, videoParams VideoGeneratorParams) paths.Path {
	args := []string{
		"-f", "lavfi",
		"-i", fmt.Sprintf("color=c=blue:s=720x608:d=%f", videoParams.Duration),
		"-vf", fmt.Sprintf("setsar=%s, setdar=%s", videoParams.SAR, videoParams.DAR),
		"-r", fmt.Sprintf("%d", videoParams.FrameRate),
		"-c:v", "prores_ks",
		"-profile:v", "3",
		"-y", outFile.Local(),
	}

	cmd := exec.Command("ffmpeg", args...)
	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	return outFile
}
