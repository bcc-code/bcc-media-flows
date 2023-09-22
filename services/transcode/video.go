package transcode

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"os"
	"path/filepath"
)

func VideoH264(input common.VideoInput, cb ffmpeg.ProgressCallback) (*common.VideoResult, error) {

	h264encoder := os.Getenv("H264_ENCODER")
	if h264encoder == "" {
		h264encoder = "libx264"
	}

	params := []string{
		"-hide_banner",
		"-progress", "pipe:1",
		"-i", input.Path,
	}

	if input.WatermarkPath != "" {
		params = append(params,
			"-i", input.WatermarkPath,
		)
	}

	switch h264encoder {
	case "libx264":
		params = append(params,
			"-c:v", h264encoder,
			"-profile:v", "high",
			"-level:v", "1.3",
			"-pix_fmt", "yuv420p",
			//"-crf", "18",
			"-maxrate", input.Bitrate,
		)
	}

	var filterComplex string

	filterComplex += fmt.Sprintf("[0] scale=%[1]d:%[2]d:force_original_aspect_ratio=decrease,pad=%[1]d:%[2]d:(ow-iw)/2:(oh-ih)/2 [main];",
		1920,
		1080,
	)

	if input.WatermarkPath != "" {
		filterComplex += "[main][1] overlay=main_w-overlay_w:0 [main];"
	}

	filterComplex += fmt.Sprintf("[main] scale=%[1]d:%[2]d:force_original_aspect_ratio=decrease [out]",
		input.Width,
		input.Height,
	)

	//if input.WatermarkPath != "" {
	//	filterComplex += fmt.Sprintf(";[main][1] overlay=main_w-overlay_w:0")
	//}

	info, err := ffmpeg.GetStreamInfo(input.Path)
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

	params = append(params,
		"-r", fmt.Sprintf("%d", framerate),
		"-filter_complex", filterComplex,
		"-map", "[out]",
	)

	filename := filepath.Base(input.Path)
	filename = filename[:len(filename)-len(filepath.Ext(filename))] +
		fmt.Sprintf("_%dx%d.mp4", input.Width, input.Height)

	outputPath := filepath.Join(input.DestinationPath, filename)

	params = append(params,
		"-y", outputPath,
	)

	_, err = ffmpeg.Do(params, info, cb)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(outputPath, os.ModePerm)
	if err != nil {
		return nil, err
	}
	return &common.VideoResult{
		OutputPath: outputPath,
	}, nil
}
