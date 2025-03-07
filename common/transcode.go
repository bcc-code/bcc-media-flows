package common

import (
	"time"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/utils"
)

type VideoInput struct {
	Path            paths.Path
	Bitrate         string
	BufferSize      string
	Resolution      utils.Resolution
	FrameRate       int
	WatermarkPath   *paths.Path
	DestinationPath paths.Path
}

type VideoResult struct {
	OutputPath paths.Path
}

type AudioInput struct {
	Path            paths.Path
	Bitrate         string
	DestinationPath paths.Path

	// Not all codecs may support constant bitrate
	ForceCBR bool
}

type DetectSilenceInput struct {
	Path         paths.Path
	SampleLength time.Duration
	offset       time.Duration
}

type WavAudioInput struct {
	Path            paths.Path
	DestinationPath paths.Path
	Timecode        string
}

type AudioResult struct {
	OutputPath paths.Path
	Bitrate    string
	Format     string
	FileSize   int64
}

// Simple muxing does not use languages
type SimpleMuxInput struct {
	FileName        string
	VideoFilePath   paths.Path
	AudioFilePaths  []paths.Path
	DestinationPath paths.Path
}

type MuxInput struct {
	FileName          string
	VideoFilePath     paths.Path
	AudioFilePaths    map[string]paths.Path
	SubtitleFilePaths map[string]paths.Path
	DestinationPath   paths.Path
}
type MuxResult struct {
	Path paths.Path
}

type AnalyzeEBUR128Result struct {
	IntegratedLoudness  float64
	TruePeak            float64
	LoudnessRange       float64
	SuggestedAdjustment float64
}

type PlayoutMuxInput struct {
	VideoFilePath     paths.Path
	AudioFilePaths    map[string]paths.Path
	SubtitleFilePaths map[string]paths.Path
	OutputDir         paths.Path
	FallbackLanguage  string
}

type PlayoutMuxResult struct {
	Path paths.Path
}
