package transcode

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/samber/lo"
)

type H264EncodeInput struct {
	FilePath       string
	OutputDir      string
	Resolution     string
	FrameRate      int
	Bitrate        string
	Interlace      bool
	BurnInSubtitle *paths.Path
	SubtitleStyle  *paths.Path
}

type EncodeResult struct {
	Path string
}

func H264(input H264EncodeInput, progressCallback ffmpeg.ProgressCallback) (*EncodeResult, error) {
	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mxf"
	outputPath := filepath.Join(input.OutputDir, filename)

	probe, err := ffmpeg.ProbeFile(input.FilePath)
	if err != nil {
		return nil, err
	}
	info := ffmpeg.ProbeResultToInfo(probe)

	h264encoder := "libx264"
	profile := "high"
	// lo if any probe.Streams has pix_fmt starting with yuv422
	if lo.SomeBy(probe.Streams, func(i ffmpeg.FFProbeStream) bool {
		return strings.HasPrefix(i.PixFmt, "yuv422")
	}) {
		profile = "high422"
	}

	params := []string{
		"-hide_banner",
		"-progress", "pipe:1",
		"-i", input.FilePath,
		"-c:v", h264encoder,
		"-ar", "48000",
	}
	switch h264encoder {
	case "libx264":
		params = append(params,
			"-profile:v", profile,
			"-level:v", "1.3",
			"-crf", "18",
		)
	}

	if input.Bitrate != "" {
		params = append(
			params,
			"-b:v", input.Bitrate,
		)
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
		assFile, err := CreateBurninASSFile(*input.SubtitleStyle, *input.BurnInSubtitle)
		if err != nil {
			return nil, err
		}
		//defer os.Remove(assFile.Local()) ??
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
