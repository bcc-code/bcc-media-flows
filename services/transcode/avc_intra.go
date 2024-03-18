package transcode

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

type AVCIntraEncodeInput struct {
	FilePath       string
	OutputDir      string
	Resolution     string
	FrameRate      int
	Interlace      bool
	BurnInSubtitle *paths.Path
}

func AvcIntra(input AVCIntraEncodeInput, progressCallback ffmpeg.ProgressCallback) (*EncodeResult, error) {
	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mxf"
	outputPath := filepath.Join(input.OutputDir, filename)

	probe, err := ffmpeg.ProbeFile(input.FilePath)
	if err != nil {
		return nil, err
	}
	info := ffmpeg.ProbeResultToInfo(probe)

	params := []string{
		"-hide_banner",
		"-progress", "pipe:1",
		"-i", input.FilePath,
		"-c:a", "pcm_s24le",
		"-c:v", "libx264",
		"-ar", "48000",
		"-b:v", "100M",
		"-pix_fmt", "yuv422p10le",
		"-x264-params", "avcintra-class=100:interlaced=0",
		"-x264opts", "colorprim=bt709",
		"-x264opts", "transfer=bt709",
		"-x264opts", "colormatrix=bt709",
	}

	if input.Resolution != "" {
		params = append(
			params,
			"-s", input.Resolution,
		)
	}

	if input.FrameRate != 0 {
		params = append(
			params,
			"-r", strconv.Itoa(input.FrameRate),
		)
	}

	var videoFilters []string

	if input.Interlace {
		params = append(
			params,
			"-flags", "+ilme+ildct",
		)
		videoFilters = append(videoFilters, "setfield=tff", "fieldorder=tff")
	} else {
		videoFilters = append(videoFilters, "yadif=0:-1:0")
	}

	if input.BurnInSubtitle != nil {
		assFile, err := CreateBurninASSFile(*input.BurnInSubtitle)
		if err != nil {
			return nil, err
		}
		videoFilters = append(videoFilters, "ass="+assFile.Local())
	}

	if len(videoFilters) > 0 {
		params = append(
			params,
			"-vf", strings.Join(videoFilters, ","),
		)
	}

	params = append(
		params,
		"-map", "v",
		"-map", "a",
		"-y",
		outputPath,
	)

	_, err = ffmpeg.Do(params, info, progressCallback)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(outputPath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	return &EncodeResult{
		Path: outputPath,
	}, nil
}
