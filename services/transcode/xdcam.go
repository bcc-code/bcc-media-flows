package transcode

import (
	"github.com/bcc-code/bcc-media-flows/utils"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

type XDCAMEncodeInput struct {
	FilePath   string
	OutputDir  string
	Resolution *utils.Resolution
	FrameRate  int
	Bitrate    string
	Interlace  bool
}

func generateFfmpegParamsForXDCAM(input XDCAMEncodeInput, output string) []string {
	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", input.FilePath,
		"-c:a", "copy",
		"-c:v", "mpeg2video",
		"-pix_fmt", "yuv422p",
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

	if input.Resolution != nil {
		params = append(
			params,
			"-s", input.Resolution.FFMpegString(),
		)
	}

	if input.FrameRate != 0 {
		params = append(
			params,
			"-r", strconv.Itoa(input.FrameRate),
		)
	}

	if input.Interlace {
		params = append(
			params,
			"-flags", "+ilme+ildct",
			"-vf", "setfield=tff,fieldorder=tff",
		)
	}

	params = append(
		params,
		output,
	)

	return params
}

func XDCAM(input XDCAMEncodeInput, progressCallback ffmpeg.ProgressCallback) (*EncodeResult, error) {
	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mxf"
	outputPath := filepath.Join(input.OutputDir, filename)
	params := generateFfmpegParamsForXDCAM(input, outputPath)

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

	return &EncodeResult{
		Path: outputPath,
	}, nil
}
