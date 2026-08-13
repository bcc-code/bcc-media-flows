package transcode

import (
	"github.com/bcc-code/bcc-media-flows/utils"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

type AVCIntraEncodeInput struct {
	FilePath       string
	OutputDir      string
	Resolution     *utils.Resolution
	FrameRate      int
	Interlace      bool
	BurnInSubtitle *paths.Path
	SubtitleStyle  *paths.Path
}

func AvcIntra(input AVCIntraEncodeInput, progressCallback ffmpeg.ProgressCallback) (*EncodeResult, error) {
	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mxf"
	outputPath := filepath.Join(input.OutputDir, filename)

	params := []string{
		"-c:a", "pcm_s24le",
		"-c:v", "libx264",
		"-ar", "48000",
		"-b:v", "100M",
		"-pix_fmt", "yuv422p10le",
		"-x264opts", "colorprim=bt709",
		"-x264opts", "transfer=bt709",
		"-x264opts", "colormatrix=bt709",
	}

	if input.Resolution != nil {
		params = append(
			params,
			"-s", input.Resolution.FFMpegString(),
		)
	}

	var videoFilters []string

	if input.FrameRate != 0 {
		videoFilters = append(
			videoFilters,
			"fps="+strconv.Itoa(input.FrameRate),
		)
	}

	if input.Interlace {
		params = append(
			params,
			"-flags", "+ilme+ildct",
			"-x264-params", "avcintra-class=100:interlaced=1:tff=1",
		)
		videoFilters = append(videoFilters, "setfield=tff", "fieldorder=tff", "interlace=tff")
	} else {
		params = append(params,
			"-x264-params", "avcintra-class=100:interlaced=0",
		)
		videoFilters = append(videoFilters, "yadif=0:-1:0")
	}

	videoFilters, err := appendBurnInFilter(videoFilters, input.SubtitleStyle, input.BurnInSubtitle)
	if err != nil {
		return nil, err
	}

	if len(videoFilters) > 0 {
		params = append(
			params,
			"-vf", strings.Join(videoFilters, ","),
		)
	}

	params = append(params, "-map", "v", "-map", "a?")

	_, err = ffmpeg.Run(ffmpeg.Job{
		Input:  input.FilePath,
		Output: outputPath,
		Args:   params,
	}, progressCallback)
	if err != nil {
		return nil, err
	}

	return &EncodeResult{
		Path: outputPath,
	}, nil
}
