package transcode

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
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

func ProRes(input ProResInput, progressCallback func(Progress)) (*ProResResult, error) {
	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mov"
	outputPath := filepath.Join(input.OutputDir, filename)

	info, err := ProbeFile(input.FilePath)
	if err != nil {
		return nil, err
	}

	commandParts := []string{
		fmt.Sprintf("-i %s", input.FilePath),
		"-c:v prores",
		"-progress pipe:1",
		"-profile:v 3",
		"-vendor ap10",
		"-vf setfield=tff",
		"-color_primaries bt709",
		"-color_trc bt709",
		"-colorspace bt709",
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
	stdout, _ := cmd.StdoutPipe()

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	cb := parseProgressCallback(infoToBase(info), progressCallback)

	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
		cb(line)
	}

	err = cmd.Wait()
	if err != nil {
		return nil, err
	}

	return &ProResResult{
		OutputPath: outputPath,
	}, nil
}
