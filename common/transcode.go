package common

type VideoInput struct {
	Path            string
	Bitrate         string
	BufferSize      string
	Width           int
	Height          int
	FrameRate       int
	WatermarkPath   string
	DestinationPath string
}

type VideoResult struct {
	OutputPath string
}

type AudioInput struct {
	Path            string
	Bitrate         string
	DestinationPath string
}

type AudioResult struct {
	OutputPath string
}

type MuxInput struct {
	FileName          string
	VideoFilePath     string
	AudioFilePaths    map[string]string
	SubtitleFilePaths map[string]string
	DestinationPath   string
}
type MuxResult struct {
	Path string
}

type AnalyzeEBUR128Result struct {
	IntegratedLoudness  float64
	TruePeak            float64
	LoudnessRange       float64
	SuggestedAdjustment float64
}

type PlayoutMuxInput struct {
	VideoFilePath     string
	AudioFilePaths    map[string]string
	SubtitleFilePaths map[string]string
	OutputDir         string
	FallbackLanguage  string
}

type PlayoutMuxResult struct {
	Path string
}
