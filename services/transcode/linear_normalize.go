package transcode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		"-c:a", "pcm_s24le", // Preserve 24-bit audio
	}

	info, err := ffmpeg.GetStreamInfo(input.Path.Local())
	if err != nil {
		return nil, err
	}

	var mapParams []string

	if len(info.VideoStreams) > 0 {
		mapParams = append(mapParams,
			"-map", "0:v",
		)
	}

	var filterParams []string

	for i, stream := range info.AudioStreams {
		if stream.Channels > 2 {
			return nil, fmt.Errorf("audio normalization not supported for %d channels", stream.Channels)
		}

		if stream.Channels == 2 {
			filterParams = append(filterParams,
				fmt.Sprintf("[0:a:%d]channelsplit=channel_layout=stereo[left%d][right%d]", i, i, i),
				fmt.Sprintf("[left%d]volume=%.2fdB[l%d]", i, adjustment, i),
				fmt.Sprintf("[right%d]volume=%.2fdB[r%d]", i, adjustment, i),
				fmt.Sprintf("[l%d][r%d]join=inputs=2:channel_layout=stereo[a%d]", i, i, i),
			)
		} else {
			filterParams = append(filterParams, fmt.Sprintf("[0:a:%d]volume=%.2fdB[a%d]", i, adjustment, i))
		}

		// Both paths produce one out stream
		mapParams = append(mapParams, "-map", fmt.Sprintf("[a%d]", i))

	}

	params = append(params, "-filter_complex", strings.Join(filterParams, ";"))
	params = append(params, mapParams...)
	params = append(params, "-y", outputFilePath)

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
