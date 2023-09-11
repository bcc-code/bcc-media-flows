package transcode

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"path/filepath"
)

func VideoH264(input common.VideoInput, cb ffmpeg.ProgressCallback) (*common.VideoResult, error) {
	params := []string{
		"-progress", "pipe:1",
		"-i", input.Path,
		"-c:v", "libx264",
		"-b:v", input.Bitrate,
		"-r", fmt.Sprintf("%d", input.FrameRate),
		"-vf", fmt.Sprintf("scale=%[1]d:%[2]d:force_original_aspect_ratio=decrease,pad=%[1]d:%[2]d:(ow-iw)/2:(oh-ih)/2",
			input.Width,
			input.Height,
		),
	}

	filename := filepath.Base(input.Path)
	filename = filename[:len(filename)-len(filepath.Ext(filename))] +
		fmt.Sprintf("_%dx%d.mp4", input.Width, input.Height)

	outputPath := filepath.Join(input.DestinationPath, filename)

	params = append(params, "-y", outputPath)

	info, err := ffmpeg.GetStreamInfo(input.Path)
	if err != nil {
		return nil, err
	}

	_, err = ffmpeg.Do(params, info, cb)
	if err != nil {
		return nil, err
	}
	return &common.VideoResult{
		OutputPath: outputPath,
	}, nil
}
