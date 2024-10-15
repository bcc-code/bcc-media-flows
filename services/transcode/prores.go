package transcode

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bcc-code/bcc-media-flows/utils"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

type ProResInput struct {
	FilePath       string
	AudioPaths     []string
	OutputDir      string
	Resolution     *utils.Resolution
	FrameRate      int
	Use4444        bool
	ForHyperdeck   bool
	BurnInSubtitle *paths.Path
	SubtitleStyle  *paths.Path
}

type ProResResult struct {
	OutputPath string
}

const (
	ProResProfileHQ   = "3"
	ProResProfile4444 = "4"
)

func ProRes(input ProResInput, progressCallback ffmpeg.ProgressCallback) (*ProResResult, error) {
	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mov"

	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", input.FilePath,
	}

	for _, i := range input.AudioPaths {
		params = append(params, "-i", i)
	}

	params = append(params,
		"-c:v", "prores_ks",
		"-vendor", "ap10",
		"-color_primaries", "bt709",
		"-color_trc", "bt709",
		"-colorspace", "bt709",
		"-bits_per_mb", "8000",
	)

	videoFilters := []string{
		"setfield=tff",
		"yadif=0:-1:0",
	}

	if input.BurnInSubtitle != nil {
		assFile, err := CreateBurninASSFile(*input.SubtitleStyle, *input.BurnInSubtitle)
		if err != nil {
			return nil, err
		}
		videoFilters = append(videoFilters, "ass="+assFile.Local())
	}

	if input.ForHyperdeck {
		params = append(
			params,
			"-pix_fmt", "yuv422p10le",
			"-profile:v", "2",
			"-flags", "+ildct+ilme",
			"-top", "1",
		)
	} else if input.Use4444 {
		params = append(
			params,
			"-pix_fmt", "yuva444p10le",
		)
		params = append(
			params,
			"-profile:v", ProResProfile4444,
		)
	} else {
		params = append(
			params,
			"-profile:v", ProResProfileHQ,
		)
	}

	if input.Resolution != nil {
		params = append(
			params,
			"-s", input.Resolution.FFMpegString(),
		)
	}

	if input.FrameRate != 0 {
		params = append(
			params,
			"-r", strconv.Itoa(input.FrameRate),
			"-video_track_timescale", strconv.Itoa(input.FrameRate),
		)
	}

	if len(videoFilters) > 0 {
		params = append(params, "-vf", strings.Join(videoFilters, ","))
	}

	outputPath := filepath.Join(input.OutputDir, filename)

	if len(input.AudioPaths) == 0 {
		params = append(
			params,
			"-map", "a?",
		)
	} else {
		params = append(
			params,
			"-c:a", "aac",
		)

		for i := range input.AudioPaths {
			params = append(
				params,
				"-map", strconv.Itoa(i+1),
			)
		}
	}

	params = append(
		params,
		"-map", "v",
		"-y",
		outputPath,
	)

	info, err := ffmpeg.GetStreamInfo(input.FilePath)
	if err != nil {
		return nil, err
	}

	_, err = ffmpeg.Do(params, info, progressCallback)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(outputPath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	return &ProResResult{
		OutputPath: outputPath,
	}, nil
}
