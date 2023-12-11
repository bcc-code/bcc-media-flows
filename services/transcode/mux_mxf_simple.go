package transcode

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bccm-flows/paths"

	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
)

// MuxToSimpleMXF multiplexes specified video and audio tracks. Video as-is but audio is enforced to 24bit 48kHz pcm. Ignores languages, etc.
func MuxToSimpleMXF(input common.SimpleMuxInput, progressCallback ffmpeg.ProgressCallback) (*common.MuxResult, error) {
	info, err := ffmpeg.GetStreamInfo(input.VideoFilePath.Local())
	if err != nil {
		return nil, err
	}

	outputFilePath := filepath.Join(input.DestinationPath.Local(), input.FileName+".mxf")

	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", input.VideoFilePath.Local(),
	}

	for _, f := range input.AudioFilePaths {
		params = append(params,
			"-i", f.Local(),
		)
	}

	streams := 0
	params = append(
		params,
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
		"-y", outputFilePath,
	)

	_, err = ffmpeg.Do(params, info, progressCallback)
	if err != nil {
		log.Default().Println("mux failed", err)
		return nil, fmt.Errorf("mux failed, %s", strings.Join(params, " "))
	}

	err = os.Chmod(outputFilePath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	outputPath, err := paths.Parse(outputFilePath)
	if err != nil {
		return nil, err
	}

	return &common.MuxResult{
		Path: outputPath,
	}, nil
}
