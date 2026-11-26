package transcribe

// Transcription is the transcriber's JSON transcript (transcript.json), in the OpenAI
// Whisper layout. Every reader of that file uses this type.
type Transcription struct {
	Text     string    `json:"text"`
	Segments []Segment `json:"segments"`
	Language string    `json:"language,omitempty"`
}

type Segment struct {
	ID               int     `json:"id"`
	Seek             int     `json:"seek"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	Tokens           []int   `json:"tokens"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
	// Confidence is nil when the transcript has none, as the transcriber's do not.
	Confidence *float64 `json:"confidence,omitempty"`
	Words      []Word   `json:"words"`
}

type Word struct {
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	// Confidence is nil when the transcript has none, as the transcriber's do not.
	Confidence *float64 `json:"confidence,omitempty"`
}
