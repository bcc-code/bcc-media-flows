package common

import (
	"github.com/bcc-code/bccm-flows/paths"
)

type VideoInput struct {
	Path            paths.Path
	Bitrate         string
	BufferSize      string
	Width           int
	Height          int
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
