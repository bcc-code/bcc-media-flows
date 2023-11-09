package transcode

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/paths"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
)

func AudioAac(input common.AudioInput, cb ffmpeg.ProgressCallback) (*common.AudioResult, error) {
	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", input.Path.Local(),
		"-c:a", "aac",
		"-b:a", input.Bitrate,
	}

	outputFilePath := filepath.Join(input.DestinationPath.Local(), input.Path.Base())
	outputFilePath = fmt.Sprintf("%s-%s.aac", outputFilePath[:len(outputFilePath)-len(filepath.Ext(outputFilePath))], input.Bitrate)

	params = append(params, "-y", outputFilePath)

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

	return &common.AudioResult{
		OutputPath: outputPath,
		Bitrate:    input.Bitrate,
		Format:     "aac",
	}, nil
}

func AudioWav(input common.AudioInput, cb ffmpeg.ProgressCallback) (*common.AudioResult, error) {
	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", input.Path.Local(),
	}

	outputFilePath := filepath.Join(input.DestinationPath.Local(), input.Path.Base())
	outputFilePath = fmt.Sprintf("%s-%s.wav", outputFilePath[:len(outputFilePath)-len(filepath.Ext(outputFilePath))], input.Bitrate)

	params = append(params, "-y", outputFilePath)

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

	return &common.AudioResult{
		OutputPath: outputPath,
		Bitrate:    input.Bitrate,
		Format:     "wav",
	}, nil
}

func getQfactorFromBitrate(input string) int {

	bitrate, _ := strconv.ParseInt(strings.ReplaceAll(input, "k", ""), 10, 64)

	switch {
	case bitrate >= 190:
		return 1
	case bitrate >= 170:
		return 2
	case bitrate >= 150:
		return 3
	case bitrate >= 140:
		return 4
	case bitrate >= 120:
		return 5
	case bitrate >= 100:
		return 6
	case bitrate >= 80:
		return 7
	case bitrate >= 70:
		return 8
	case bitrate >= 45:
		return 9
	default:
		return 1
	}
}

func AudioMP3(input common.AudioInput, cb ffmpeg.ProgressCallback) (*common.AudioResult, error) {
	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", input.Path.Local(),
		"-c:a", "libmp3lame",
		"-q:a", fmt.Sprint(getQfactorFromBitrate(input.Bitrate)),
	}

	outputFilePath := filepath.Join(input.DestinationPath.Local(), input.Path.Base())
	outputFilePath = fmt.Sprintf("%s-%s.mp3", outputFilePath[:len(outputFilePath)-len(filepath.Ext(outputFilePath))], input.Bitrate)

	params = append(params, "-y", outputFilePath)

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

	return &common.AudioResult{
		OutputPath: outputPath,
		Bitrate:    input.Bitrate,
		Format:     "mp3",
	}, nil
}
