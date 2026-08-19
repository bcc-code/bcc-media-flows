package transcode

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bcc-media-flows/paths"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

// MuxToSimpleMXF multiplexes specified video and audio tracks. Video as-is but audio is enforced to 24bit 48kHz pcm. Ignores languages, etc.
func MuxToSimpleMXF(input common.SimpleMuxInput, progressCallback ffmpeg.ProgressCallback) (*common.MuxResult, error) {
	info, err := ffmpeg.GetStreamInfo(input.VideoFilePath.Local())
	if err != nil {
		return nil, err
	}

	outputFilePath := filepath.Join(input.DestinationPath.Local(), input.FileName+".mxf")

	var extraInputs []ffmpeg.Input
	for _, f := range input.AudioFilePaths {
		extraInputs = append(extraInputs, ffmpeg.Input{Path: f.Local()})
	}

	streams := 0
	params := append(
		[]string{},
		"-map", fmt.Sprintf("%d:v", streams),
	)
	streams++

	for range input.AudioFilePaths {
		params = append(params,
			"-map", fmt.Sprintf("%d:a", streams),
		)
		streams++
	}

	params = append(params,
		"-c:v", "copy",
		"-ar", "48000",
		"-c:a", "pcm_s24le",
	)

	job := ffmpeg.Job{
		Input:       input.VideoFilePath.Local(),
		ExtraInputs: extraInputs,
		Output:      outputFilePath,
		Args:        params,
		Info:        &info,
	}

	_, err = ffmpeg.Run(job, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("mux failed (%s): %w", strings.Join(job.Arguments(), " "), err)
	}

	outputPath, err := paths.Parse(outputFilePath)
	if err != nil {
		return nil, err
	}

	return &common.MuxResult{
		Path: outputPath,
	}, nil
}
