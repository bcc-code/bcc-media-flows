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

	blankFile := paths.Path{
		Drive: paths.IsilonDrive,
		Path:  "system/assets/BlankVideo10h.mxf",
	}

	info, err := ffmpeg.GetStreamInfo(files[0].Local())
	if err != nil {
		return nil, err
	}

	params := []string{
		"-i", blankFile.Local(),
		"-t", fmt.Sprintf("%f", info.TotalSeconds),
	}

	for _, f := range files {
		params = append(params, "-i", f.Local())
	}

	params = append(params,
		"-vf", fmt.Sprintf("scale=960:540:force_original_aspect_ratio=decrease,pad=960:540:(ow-iw)/2:(oh-ih)/2,drawtext=text=%s:fontsize=36:fontcolor=white:x=100:y=100", text),
		"-c:v", "libx264",
		"-c:a", "pcm_s24le",
		outputPath.Append(files[0].Base()+".mxf").Local(),
	)

	_, err = ffmpeg.Do(params, ffmpeg.StreamInfo{}, cb)
	if err != nil {
		return nil, err
	}
	return &outputPath, nil
}
