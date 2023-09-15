package transcode

import (
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"os"
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

func H264(input EncodeInput, progressCallback ffmpeg.ProgressCallback) (*EncodeResult, error) {
	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mxf"
	outputPath := filepath.Join(input.OutputDir, filename)

	info, err := ffmpeg.GetStreamInfo(input.FilePath)
	if err != nil {
		return nil, err
	}

	h264encoder := "libx264"

	params := []string{
		"-hide_banner",
		"-progress", "pipe:1",
		"-i", input.FilePath,
		"-vf", "yadif=0:-1:0",
		"-c:v", h264encoder,
	}
	switch h264encoder {
	case "libx264":
		params = append(params,
			"-profile:v", "high",
			"-level:v", "1.3",
			"-crf", "18",
		)
	}

	if input.Bitrate != "" {
		params = append(
			params,
			"-b:v", input.Bitrate,
		)
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

	params = append(
		params,
		"-y",
		outputPath,
	)

	_, err = ffmpeg.Do(params, info, progressCallback)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(outputPath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	return &EncodeResult{
		Path: outputPath,
	}, nil
}
