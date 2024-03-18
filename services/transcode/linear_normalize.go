package transcode

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bcc-code/bcc-media-flows/paths"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

// AdjustAudioLevel adjusts the audio level of the input file by the given adjustment in dB
// without changing the dynamic range. This function does not protect against clipping!
func AdjustAudioLevel(input common.AudioInput, adjustment float64, cb ffmpeg.ProgressCallback) (*common.AudioResult, error) {
	outputFilePath := filepath.Join(input.DestinationPath.Local(), input.Path.Base())
	outputFilePath = outputFilePath[:len(outputFilePath)-len(filepath.Ext(outputFilePath))] + "_normalized" + filepath.Ext(outputFilePath)

	params := []string{
		"-i", input.Path.Local(),
		"-c:v", "copy",
		"-af", fmt.Sprintf("volume=%.2fdB", adjustment),
		outputFilePath,
	}

	info, err := ffmpeg.GetStreamInfo(input.Path.Local())
	if err != nil {
		return nil, err
	}

	_, err = ffmpeg.Do(params, info, cb)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(outputFilePath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	outputPath, err := paths.Parse(outputFilePath)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(outputFilePath)
	if err != nil {
		return nil, err
	}

	return &common.AudioResult{
		OutputPath: outputPath,
		FileSize:   fileInfo.Size(),
	}, nil
}
