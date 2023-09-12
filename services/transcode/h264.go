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

	h264encoder := os.Getenv("H264_ENCODER")
	if h264encoder == "" {
		h264encoder = "libx264"
	}

	params := []string{
		"-i", input.FilePath,
		"-c:v", h264encoder,
		"-progress", "pipe:1",
		"-profile:v", "high422",
		"-pix_fmt", "yuv422p10le",
		"-vf", "setfield=tff,format=yuv422p10le",
		"-color_primaries", "bt709",
		"-color_trc", "bt709",
		"-colorspace", "bt709",
		"-y",
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
