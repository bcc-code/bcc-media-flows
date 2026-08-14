package transcode

import (
	"github.com/bcc-code/bcc-media-flows/utils"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

type XDCAMEncodeInput struct {
	FilePath   string
	OutputDir  string
	Resolution *utils.Resolution
	FrameRate  int
	Bitrate    string
	Interlace  bool
}

// xdcamArgs returns the codec arguments, i.e. everything ffmpeg.Job puts
// between the input and the output.
func xdcamArgs(input XDCAMEncodeInput) []string {
	params := []string{
		"-c:a", "copy",
		"-c:v", "mpeg2video",
		"-pix_fmt", "yuv422p",
		"-color_primaries", "bt709",
		"-color_trc", "bt709",
		"-colorspace", "bt709",
	}

	if input.Bitrate != "" {
		params = append(
			params,
			"-b:v", input.Bitrate,
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
		)
	}

	if input.Interlace {
		params = append(
			params,
			"-flags", "+ilme+ildct",
			"-vf", "setfield=tff,fieldorder=tff",
		)
	}

	return params
}

func XDCAM(input XDCAMEncodeInput, progressCallback ffmpeg.ProgressCallback) (*EncodeResult, error) {
	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mxf"
	outputPath := filepath.Join(input.OutputDir, filename)

	_, err := ffmpeg.Run(ffmpeg.Job{
		Input:  input.FilePath,
		Output: outputPath,
		Args:   xdcamArgs(input),
	}, progressCallback)
	if err != nil {
		return nil, err
	}

	return &EncodeResult{
		Path: outputPath,
	}, nil
}
