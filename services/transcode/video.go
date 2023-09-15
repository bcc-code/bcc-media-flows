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
		"-vf", "yadif=0:-1:0",
		"-c:v", h264encoder,
	}
	switch h264encoder {
	case "libx264":
		params = append(params,
			"-profile:v", "high",
			"-level:v", "1.3",
			"-crf", "18",
			"-maxrate", input.Bitrate,
		)
	}

	params = append(params,
		"-r", fmt.Sprintf("%d", input.FrameRate),
		"-vf", fmt.Sprintf("scale=%[1]d:%[2]d:force_original_aspect_ratio=decrease,pad=%[1]d:%[2]d:(ow-iw)/2:(oh-ih)/2",
			input.Width,
			input.Height,
		),
	)

	filename := filepath.Base(input.Path)
	filename = filename[:len(filename)-len(filepath.Ext(filename))] +
		fmt.Sprintf("_%dx%d.mp4", input.Width, input.Height)

	outputPath := filepath.Join(input.DestinationPath, filename)

	params = append(params,
		"-y", outputPath,
	)

	info, err := ffmpeg.GetStreamInfo(input.Path)
	if err != nil {
		return nil, err
	}

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
