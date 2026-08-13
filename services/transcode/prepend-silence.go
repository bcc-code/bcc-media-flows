package transcode

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

// PrependSilence prepends a given file with a given length of silence.
// The sample rate of the output file is the same as the input file.
func PrependSilence(file paths.Path, outputPath paths.Path, length float64, sampleRate int, cb ffmpeg.ProgressCallback) (*paths.Path, error) {
	_, err := ffmpeg.Run(ffmpeg.Job{
		// The silence is synthesised, so -f lavfi describes the first input; the
		// real file follows it and the concat filter refers to the two as 0 and 1.
		InputArgs:   []string{"-f", "lavfi"},
		Input:       fmt.Sprintf("aevalsrc=0|0:d=%f", length),
		ExtraInputs: []ffmpeg.Input{{Path: file.Local()}},
		Output:      outputPath.Local(),
		Args: []string{
			"-filter_complex", "[0:a][1:a]concat=n=2:v=0:a=1",
			"-ar", fmt.Sprintf("%d", sampleRate),
		},
		Info: &ffmpeg.StreamInfo{},
	}, cb)
	if err != nil {
		return nil, err
	}
	return &outputPath, nil
}
