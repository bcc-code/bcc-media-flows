package transcode

import (
	"fmt"
	"strings"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

func MultitrackMux(files paths.Files, outputPath paths.Path, cb ffmpeg.ProgressCallback) (*paths.Path, error) {
	lines := []string{
		"Multitrack VB",
		"",
	}

	for _, f := range files {
		lines = append(lines, f.Base())
	}

	text := strings.Join(lines, "\n")

	info, err := ffmpeg.GetStreamInfo(files[0].Local())
	if err != nil {
		return nil, err
	}

	params := []string{
		"-f", "lavfi", "-i", fmt.Sprintf("color=c=black:s=1920x1080:r=25:d=%f", info.TotalSeconds),
	}

	for _, f := range files {
		params = append(params, "-i", f.Local())
	}

	outputPath = outputPath.Append(files[0].Base() + ".mxf")

	params = append(params, "-map", "v")
	for i, _ := range files {
		t := i + 1
		params = append(params, "-filter_complex", fmt.Sprintf("[%d:a:0]channelsplit=channel_layout=stereo[l%d][r%d]", t, t, t))
		params = append(params, "-map", fmt.Sprintf("[l%d]", t))
		params = append(params, "-map", fmt.Sprintf("[r%d]", t))
	}

	params = append(params,
		"-vf", fmt.Sprintf("scale=960:540:force_original_aspect_ratio=decrease,pad=960:540:(ow-iw)/2:(oh-ih)/2,drawtext=text=%s:fontsize=36:fontcolor=white:x=100:y=100", text),
		"-c:v", "libx264",
		"-c:a", "pcm_s24le",
		"-t", fmt.Sprintf("%f", info.TotalSeconds),
		"-y",
		outputPath.Local(),
	)

	_, err = ffmpeg.Do(params, ffmpeg.StreamInfo{}, cb)
	if err != nil {
		return nil, err
	}

	return &outputPath, nil
}
