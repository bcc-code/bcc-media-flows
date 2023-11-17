package transcode

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
)

func VideoH264(input common.VideoInput, cb ffmpeg.ProgressCallback) (*common.VideoResult, error) {
	h264encoder := os.Getenv("H264_ENCODER")
	h264encoder = "libx264"

	params := []string{
		"-hide_banner",
		"-progress", "pipe:1",
		"-i", input.Path.Local(),
	}

	if input.WatermarkPath != nil {
		params = append(params,
			"-i", input.WatermarkPath.Local(),
		)
	}

	switch h264encoder {
	case "libx264":
		params = append(params,
			"-c:v", h264encoder,
			"-profile:v", "high422",
			"-preset", "slow",
			"-level:v", "1.3",
			"-tune", "film",
			"-vsync", "1",
			"-g", "48",
			"-pix_fmt", "yuv420p",
			"-x264opts", "no-scenecut",
			"-crf", "22",
			"-write_tmcd", "0",
		)
	}

	info, err := ffmpeg.GetStreamInfo(input.Path.Local())
	if err != nil {
		return nil, err
	}

	framerate := input.FrameRate
	if framerate == 0 {
		if info.FrameRate > 40 {
			framerate = 50
		} else {
			framerate = 25
		}
	}

	var filterComplex string

	if input.WatermarkPath != nil {
		filterComplex += "[0:0][1:0]overlay=main_w-overlay_w:0[main];"
	} else {
		filterComplex += "[0:0]copy[main];"
	}

	height := input.Height
	width := -1
	if info.Height != 0 && info.Width != 0 && info.Height > info.Width {
		// portrait video
		height = -1
		width = input.Height
	}

	filterComplex += fmt.Sprintf("[main]scale=%[1]d:%[2]d:force_original_aspect_ratio=decrease[out]", width, height)

	params = append(params,
		"-filter_complex", filterComplex,
		"-map", "[out]",
	)

	params = append(params,
		"-r", fmt.Sprintf("%d", framerate),
	)

	filename := input.Path.Base()
	filename = filename[:len(filename)-len(filepath.Ext(filename))] +
		fmt.Sprintf("_%dx%d.mp4", input.Width, input.Height)

	outputFilePath := filepath.Join(input.DestinationPath.Local(), filename)

	params = append(params,
		"-y", outputFilePath,
	)

	_, err = ffmpeg.Do(params, info, cb)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(outputFilePath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	outputPath, err := paths.Parse(outputFilePath)
	if err != nil {
		return nil, err
	}

	return &common.VideoResult{
		OutputPath: outputPath,
	}, nil
}
