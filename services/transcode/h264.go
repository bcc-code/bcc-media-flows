package transcode

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/utils"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type EncodeInput struct {
	FilePath   string
	OutputDir  string
	Resolution string
	FrameRate  int
	Bitrate    string
}

type EncodeResult struct {
	Path string
}

func H264(input EncodeInput, progressCallback func(float64)) (*EncodeResult, error) {
	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mxf"
	outputPath := filepath.Join(input.OutputDir, filename)

	info, err := ProbeFile(input.FilePath)
	if err != nil {
		return nil, err
	}

	commandParts := []string{
		fmt.Sprintf("-i %s", input.FilePath),
		"-c:v libx264",
		"-progress pipe:1",
		"-profile:v high422",
		"-pix_fmt yuv422p10le",
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

	return &EncodeResult{
		Path: outputPath,
	}, nil
}
