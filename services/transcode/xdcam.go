package transcode

import (
	"bufio"
	"fmt"
	"github.com/pkg/errors"
	"os/exec"
	"path/filepath"
	"strings"
)

func XDCAM(input EncodeInput, progressCallback func(float64)) (*EncodeResult, error) {
	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mxf"
	outputPath := filepath.Join(input.OutputDir, filename)

	info, err := ProbeFile(input.FilePath)
	if err != nil {
		return nil, err
	}

	commandParts := []string{
		fmt.Sprintf("-i %s", input.FilePath),
		"-c:v mpeg2video",
		"-progress pipe:1",
		"-pix_fmt yuv422p",
		"-color_primaries bt709",
		"-color_trc bt709",
		"-colorspace bt709",
		"-y",
	}

	if input.Bitrate != "" {
		commandParts = append(
			commandParts,
			fmt.Sprintf("-b:v %s", input.Bitrate),
		)
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
		return nil, errors.Wrap(err, "couldn't start ffmpeg")
	}

	cb := parseProgressCallback(info, progressCallback)

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

	return &EncodeResult{
		Path: outputPath,
	}, nil
}
