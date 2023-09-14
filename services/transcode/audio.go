package transcode

import (
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"os"
	"path/filepath"
)

func AudioAac(input common.AudioInput, cb ffmpeg.ProgressCallback) (*common.AudioResult, error) {
	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", input.Path,
		"-c:a", "aac",
		"-b:a", input.Bitrate,
	}

	outputPath := filepath.Join(input.DestinationPath, filepath.Base(input.Path))

	//replace output extension to .aac
	outputPath = outputPath[:len(outputPath)-len(filepath.Ext(outputPath))] + ".aac"

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
