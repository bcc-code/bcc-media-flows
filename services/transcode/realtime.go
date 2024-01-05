package transcode

import (
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

func RealtimePreview(in paths.Path, out paths.Path) error {
	params := []string{
		"-re",
		"-i", in.Local(),
		"-filter_complex", "[0:v]scale=-2:720",
		"-c:v", "libx264",
		"-c:a", "aac",
		"-preset", "ultrafast",
		"-f", "flv",
		out.Local(),
	}

	_, err := ffmpeg.Do(params, ffmpeg.StreamInfo{}, nil)
	if err != nil {
		return err
	}

	return nil
}
