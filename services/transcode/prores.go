package transcode

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bcc-code/bcc-media-flows/utils"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/orsinium-labs/enum"
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

// ProResProfile is the -profile:v value ffmpeg's prores_ks encoder takes.
type ProResProfile enum.Member[string]

var (
	ProResProfileHQ   = ProResProfile{Value: "3"}
	ProResProfile4444 = ProResProfile{Value: "4"}
	ProResProfiles    = enum.New(ProResProfileHQ, ProResProfile4444)
)

func ProRes(input ProResInput, progressCallback ffmpeg.ProgressCallback) (*EncodeResult, error) {
	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mov"

	var params []string

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

	videoFilters, err := appendBurnInFilter(videoFilters, input.SubtitleStyle, input.BurnInSubtitle)
	if err != nil {
		return nil, err
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
			"-profile:v", ProResProfile4444.Value,
		)
	} else {
		params = append(
			params,
			"-profile:v", ProResProfileHQ.Value,
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

	params = append(params, "-map", "v")

	_, err = ffmpeg.Run(ffmpeg.Job{
		Input:       input.FilePath,
		ExtraInputs: ffmpeg.FileInputs(input.AudioPaths),
		Output:      outputPath,
		Args:        params,
	}, progressCallback)
	if err != nil {
		return nil, err
	}

	return &EncodeResult{
		Path: outputPath,
	}, nil
}
