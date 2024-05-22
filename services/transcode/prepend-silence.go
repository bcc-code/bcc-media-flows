package transcode

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

// PrependSilence prepends a given file with a given length of silence.
// The sample rate of the output file is the same as the input file.
func PrependSilence(file paths.Path, outputPath paths.Path, length float64, sampleRate int, cb ffmpeg.ProgressCallback) (*paths.Path, error) {
	params := []string{
		"-f", "lavfi",
		"-i", fmt.Sprintf("aevalsrc=0|0:d=%f", length),
		"-i", file.Local(),
		"-filter_complex", fmt.Sprintf("[0:a][1:a]concat=n=2:v=0:a=1"),
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-y",
		outputPath.Local(),
	}

	_, err := ffmpeg.Do(params, ffmpeg.StreamInfo{}, cb)
	if err != nil {
		return nil, err
	}
	return &outputPath, nil
}
