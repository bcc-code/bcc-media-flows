package transcode

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"os"
	"path/filepath"
)

func VideoH264(input common.VideoInput, cb ffmpeg.ProgressCallback) (*common.VideoResult, error) {
	// -hide_banner -progress pipe:1 -i /mnt/isilon/system/tmp/workflows/2c402aa0-da5e-43a1-a321-dc8d255efe90/BIST_S01_E04_SEQ.mxf -i /mnt/isilon/system/assets/BTV_LOGO_WATERMARK_BUG_GFX_1080.png
	// -c:v libx264 -profile:v high -preset veryfast -level:v 1.3 -tune film -vsync 1 -g 48 -pix_fmt yuv420p -x264opts no-scenecut -b:v 8M -maxrate 8M -bufsize 5M -r 25
	// -filter_complex [0] scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2 [main];[main][1] overlay=main_w-overlay_w:0 [main];[main] scale=1920:1080:force_original_aspect_ratio=decrease [out]
	// -map [out]
	// -y /mnt/isilon/system/tmp/workflows/2c402aa0-da5e-43a1-a321-dc8d255efe90/BIST_S01_E04_SEQ_1920x1080.mp4

	h264encoder := os.Getenv("H264_ENCODER")
	h264encoder = "libx264"

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
			"-profile:v", "high422",
			"-preset", "slow",
			"-level:v", "1.3",
			"-tune", "film",
			"-vsync", "1",
			"-g", "48",
			"-pix_fmt", "yuv420p",
			"-x264opts", "no-scenecut",
			"-crf", "18",
		)
	case "libx265":
		params = append(params,
			"-c:v", h264encoder,
			"-profile:v", "main",
			"-pix_fmt", "yuv420p",
			//"-crf", "18",
		)
	}

	params = append(params,
		"-crf", "20",
		//"-b:v", input.Bitrate,
		//"-maxrate", input.Bitrate,
	)

	//if input.BufferSize != "" {
	//	params = append(params,
	//		"-bufsize", input.BufferSize,
	//	)
	//}

	var filterComplex string

	filterComplex += fmt.Sprintf("[0] scale=%[1]d:%[2]d:force_original_aspect_ratio=decrease,pad=%[1]d:%[2]d:(ow-iw)/2:(oh-ih)/2 [main];",
		1920,
		1080)

	if input.WatermarkPath != "" {
		filterComplex += "[main][1] overlay=main_w-overlay_w:0 [main];"
	}

	filterComplex += fmt.Sprintf("[main] scale=%[1]d:%[2]d:force_original_aspect_ratio=decrease [out]",
		input.Width,
		input.Height)

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
