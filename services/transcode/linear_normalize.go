package transcode

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
)

func LinearNormalizeAudio(input common.AudioInput, adjustment float64, cb ffmpeg.ProgressCallback) (*common.AudioResult, error) {
	outputPath := filepath.Join(input.DestinationPath, filepath.Base(input.Path))
	outputPath = outputPath[:len(outputPath)-len(filepath.Ext(outputPath))] + "_normalized" + filepath.Ext(outputPath)

	params := []string{
		"-i", input.Path,
		"-c:v", "copy",
		"-af", fmt.Sprintf("volume=%.2fdB", adjustment),
		outputPath,
	}

	//replace output extension to .aac
	params = append(params, "-y", outputPath)

	info, err := ffmpeg.GetStreamInfo(input.Path)
	if err != nil {
		return nil, err
	}

	_, err = ffmpeg.Do(params, info, cb)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(outputPath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	return &common.AudioResult{
		OutputPath: outputPath,
	}, nil
}
