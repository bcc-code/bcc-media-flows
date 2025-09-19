package transcode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

type HAPInput struct {
	FilePath  string
	OutputDir string
}

type HAPResult struct {
	OutputPath string
}

func HAP(input HAPInput, progressCallback ffmpeg.ProgressCallback) (*HAPResult, error) {
	info, err := ffmpeg.GetStreamInfo(input.FilePath)
	if err != nil {
		return nil, err
	}

	if !info.HasVideo {
		return nil, fmt.Errorf("input file has no video stream")
	}

	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mov"

	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", input.FilePath,
		"-c:v", "hap",
		"-format", "hap_q",
		"-r", "50",
		"-map", "0:v:0",
	}

	if info.HasAudio {
		params = append(params, "-c:a", "copy", "-map", "0:a")
	}

	outputPath := filepath.Join(input.OutputDir, filename)
	params = append(params, "-y", outputPath)

	_, err = ffmpeg.Do(params, info, progressCallback)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(outputPath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	return &HAPResult{
		OutputPath: outputPath,
	}, nil
}