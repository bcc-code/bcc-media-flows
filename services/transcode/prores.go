package transcode

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/utils"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type ProResInput struct {
	FilePath   string
	OutputDir  string
	Resolution string
	FrameRate  int
}

type ProResResult struct {
	OutputPath string
}

func parseProgressCallback(totalFrames float64, cb func(float64)) func(string) {
	return func(line string) {

		parts := strings.Split(line, "=")

		if len(parts) != 2 {
			return
		}

		if parts[0] == "frame" {
			frame, _ := strconv.ParseFloat(parts[1], 64)
			if frame == 0 {
				cb(0)
			} else {
				cb(frame / totalFrames)
			}
		}
		if parts[0] == "progress" {
			// Audio doesn't report progress in a conceivable way, so just return 1 on complete
			if parts[1] == "end" {
				cb(1)
			}
		}
	}
}

func ProRes(input ProResInput, progressCallback func(float64)) (*ProResResult, error) {
	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mov"
	outputPath := filepath.Join(input.OutputDir, filename)

	info, err := ProbeFile(input.FilePath)
	if err != nil {
		return nil, err
	}

	commandParts := []string{
		fmt.Sprintf("-i %s", input.FilePath),
		"-c:v prores_ks",
		"-progress pipe:1",
		"-profile:v 3",
		"-vendor ap10",
		"-y",
	}

	if input.Resolution != "" {
		commandParts = append(
			commandParts,
			fmt.Sprintf("-s %s", input.Resolution),
		)
	}

	if input.FrameRate != 0 {
		commandParts = append(
			commandParts,
			fmt.Sprintf("-r %d", input.FrameRate),
		)
	}

	commandParts = append(
		commandParts,
		"-bits_per_mb 8000",
		outputPath,
	)

	commandString := strings.Join(commandParts, " ")

	cmd := exec.Command("ffmpeg",
		strings.Split(
			commandString,
			" ",
		)...,
	)

	totalFrames, err := strconv.ParseFloat(info.Streams[0].NbFrames, 64)
	if err != nil {
		return nil, err
	}

	_, err = utils.ExecuteCmd(cmd, parseProgressCallback(totalFrames, progressCallback))
	if err != nil {
		return nil, err
	}

	return &ProResResult{
		OutputPath: outputPath,
	}, nil
}
