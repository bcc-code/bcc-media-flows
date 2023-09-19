package transcode

import (
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"os"
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

func ProRes(input ProResInput, progressCallback ffmpeg.ProgressCallback) (*ProResResult, error) {
	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mov"

	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", input.FilePath,
		"-c:v", "prores",
		"-profile:v", "3",
		"-vendor", "ap10",
		"-vf", "setfield=tff",
		"-color_primaries", "bt709",
		"-color_trc", "bt709",
		"-colorspace", "bt709",
		"-bits_per_mb", "8000",
	}

	if input.Resolution != "" {
		params = append(
			params,
			"-s", input.Resolution,
		)
	}

	if input.FrameRate != 0 {
		params = append(
			params,
			"-r", strconv.Itoa(input.FrameRate),
		)
	}

	outputPath := filepath.Join(input.OutputDir, filename)
	params = append(
		params,
		"-y",
		outputPath,
	)

	info, err := ffmpeg.GetStreamInfo(input.FilePath)
	if err != nil {
		return nil, err
	}

	_, err = ffmpeg.Do(params, info, progressCallback)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(outputPath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	return &ProResResult{
		OutputPath: outputPath,
	}, nil
}
