package mediabrain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/bcc-code/bcc-media-flows/services/srt"
	"github.com/bcc-code/bcc-media-flows/services/transcribe"
)

const (
	// MaxContentBytes is the largest content Brain imports.
	MaxContentBytes = 5 << 20
	// MaxWordsPerCue is the most words Brain stores with one cue.
	MaxWordsPerCue = 1000
)

// Transcript is the JSON content Brain imports with SubtitleFormatJSON: one cue per
// segment, each with the words spoken in it. Times are in seconds.
type Transcript struct {
	Segments []TranscriptSegment `json:"segments"`
}

type TranscriptSegment struct {
	Start float64          `json:"start"`
	End   float64          `json:"end"`
	Text  string           `json:"text"`
	Words []TranscriptWord `json:"words,omitempty"`
}

type TranscriptWord struct {
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	// Nil when the transcriber reported none.
	Confidence *float64 `json:"confidence,omitempty"`
}

// ReadWords returns the words of all segments of the transcriber's JSON, ordered by
// start. Words without text are left out.
func ReadWords(whisperJSON []byte) ([]TranscriptWord, error) {
	var t transcribe.Transcription
	if err := json.Unmarshal(whisperJSON, &t); err != nil {
		return nil, fmt.Errorf("reading transcript words: %w", err)
	}

	var words []TranscriptWord
	for _, s := range t.Segments {
		for _, w := range s.Words {
			text := strings.TrimSpace(w.Text)
			if text == "" {
				continue
			}
			words = append(words, TranscriptWord{Text: text, Start: w.Start, End: w.End, Confidence: w.Confidence})
		}
	}
	sort.SliceStable(words, func(i, j int) bool { return words[i].Start < words[j].Start })
	return words, nil
}

// WordFixes counts what NewTranscript changed so that Brain accepts the words.
type WordFixes struct {
	// StartsClamped is how many negative starts became 0.
	StartsClamped int
	// EndsRaised is how many ends before their start became the start.
	EndsRaised int
	// ConfidencesDropped is how many confidences outside [0, 1] were left out.
	ConfidencesDropped int
	// WordsDropped is how many words were left out because their cue got more than
	// MaxWordsPerCue.
	WordsDropped int
}

func (f WordFixes) Total() int {
	return f.StartsClamped + f.EndsRaised + f.ConfidencesDropped + f.WordsDropped
}

// NewTranscript makes one segment per cue and gives each word to the last cue that
// starts at or before it; words before the first cue go to the first cue. cues must be
// ordered by start and acceptable to Brain, as srt.Repair leaves them. Brain refuses the
// whole import for one bad word, so words are changed until it accepts them, and the
// changes are counted. A cue that would get more than MaxWordsPerCue gets none.
func NewTranscript(cues []srt.Cue, words []TranscriptWord) (Transcript, WordFixes) {
	t := Transcript{Segments: make([]TranscriptSegment, len(cues))}
	for i, c := range cues {
		t.Segments[i] = TranscriptSegment{
			Start: float64(c.Start) / 1000,
			End:   float64(c.End) / 1000,
			Text:  c.Text,
		}
	}

	var fixes WordFixes
	if len(cues) == 0 {
		return t, fixes
	}

	for _, w := range words {
		if w.Start < 0 {
			w.Start = 0
			fixes.StartsClamped++
		}
		if w.End < w.Start {
			w.End = w.Start
			fixes.EndsRaised++
		}
		if w.Confidence != nil && !(*w.Confidence >= 0 && *w.Confidence <= 1) {
			w.Confidence = nil
			fixes.ConfidencesDropped++
		}

		// Compared in milliseconds, as Brain stores both.
		startMs := srt.SecondsToMillis(w.Start)
		i := sort.Search(len(cues), func(i int) bool { return cues[i].Start > startMs }) - 1
		i = max(i, 0)
		t.Segments[i].Words = append(t.Segments[i].Words, w)
	}

	for i := range t.Segments {
		if n := len(t.Segments[i].Words); n > MaxWordsPerCue {
			fixes.WordsDropped += n
			t.Segments[i].Words = nil
		}
	}

	return t, fixes
}

// WithoutWords is the transcript with only its cues.
func (t Transcript) WithoutWords() Transcript {
	out := Transcript{Segments: make([]TranscriptSegment, len(t.Segments))}
	for i, s := range t.Segments {
		s.Words = nil
		out.Segments[i] = s
	}
	return out
}
