package transcode

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/utils"
	"os/exec"
	"path/filepath"
	"strings"
)

type ProResInput struct {
	FilePath  string
	OutputDir string
}

type ProResResult struct {
	OutputPath string
}

func ProRes(input ProResInput) (*ProResResult, error) {
	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mov"
	outputPath := filepath.Join(input.OutputDir, filename)

	commandParts := []string{
		fmt.Sprintf("-i %s", input.FilePath),
		"-c:v prores_ks",
		"-profile:v 3",
		"-vendor ap10",
		"-y",
		"-bits_per_mb 8000",
		outputPath,
	}

	commandString := strings.Join(commandParts, " ")

	cmd := exec.Command("ffmpeg",
		strings.Split(
			commandString,
			" ",
		)...,
	)

	_, err := utils.ExecuteCmd(cmd, nil)
	if err != nil {
		return nil, err
	}

	return &ProResResult{
		OutputPath: outputPath,
	}, nil
}
