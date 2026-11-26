package mediabrain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bcc-code/bcc-media-flows/services/srt"
)

func ptr(f float64) *float64 { return &f }

func TestReadWords(t *testing.T) {
	words, err := ReadWords([]byte(`{"text": "Hei verden", "segments": [
		{"start": 3, "end": 5, "text": "verden", "words": [
			{"text": " verden", "start": 3.2, "end": 3.9, "confidence": 0.8}
		]},
		{"start": 1, "end": 2, "text": "Hei", "words": [
			{"text": "Hei", "start": 1.0, "end": 1.4, "confidence": 0.93},
			{"text": "  ", "start": 1.5, "end": 1.6}
		]},
		{"start": 6, "end": 7, "text": "slutt"}
	]}`))
	require.NoError(t, err)

	assert.Equal(t, []TranscriptWord{
		{Text: "Hei", Start: 1.0, End: 1.4, Confidence: ptr(0.93)},
		{Text: "verden", Start: 3.2, End: 3.9, Confidence: ptr(0.8)},
	}, words)
}

func TestReadWords_NoConfidence(t *testing.T) {
	words, err := ReadWords([]byte(`{"segments": [{"words": [{"text": "Hei", "start": 1, "end": 2}]}]}`))
	require.NoError(t, err)
	require.Len(t, words, 1)
	assert.Nil(t, words[0].Confidence)
}

func TestReadWords_Broken(t *testing.T) {
	_, err := ReadWords([]byte(`{"segments": [`))
	assert.Error(t, err)
}

var cues = []srt.Cue{
	{Start: 1000, End: 2000, Text: "Hei"},
	{Start: 3000, End: 6000, Text: "store\nverden"},
	{Start: 6000, End: 7000, Text: "slutt"},
}

func TestNewTranscript_AssignsWordsToCues(t *testing.T) {
	words := []TranscriptWord{
		{Text: "før", Start: 0.2, End: 0.5},
		{Text: "Hei", Start: 1.0, End: 1.4},
		{Text: "pause", Start: 2.5, End: 2.8},
		{Text: "store", Start: 2.9996, End: 3.5}, // rounds to the cue's start
		{Text: "verden", Start: 4, End: 6.5},     // may run past its cue
		{Text: "slutt", Start: 6, End: 7},
		{Text: "etter", Start: 9, End: 10},
	}

	got, fixes := NewTranscript(cues, words)

	assert.Zero(t, fixes)
	assert.Equal(t, Transcript{Segments: []TranscriptSegment{
		{Start: 1, End: 2, Text: "Hei", Words: words[0:3]},
		{Start: 3, End: 6, Text: "store\nverden", Words: words[3:5]},
		{Start: 6, End: 7, Text: "slutt", Words: words[5:7]},
	}}, got)
}

// A word a fraction of a millisecond before a cue goes to the cue it rounds into.
func TestNewTranscript_WordStartingWithCueByMillis(t *testing.T) {
	got, _ := NewTranscript(cues, []TranscriptWord{{Text: "store", Start: 2.9996, End: 3.5}})
	assert.Empty(t, got.Segments[0].Words)
	assert.Len(t, got.Segments[1].Words, 1)
}

func TestNewTranscript_MakesWordsSafe(t *testing.T) {
	got, fixes := NewTranscript(cues, []TranscriptWord{
		{Text: "negativ", Start: -0.3, End: -0.1, Confidence: ptr(1.2)},
		{Text: "baklengs", Start: 1.5, End: 1.2, Confidence: ptr(-0.1)},
		{Text: "grense", Start: 1.6, End: 1.7, Confidence: ptr(1)},
	})

	assert.Equal(t, WordFixes{StartsClamped: 1, EndsRaised: 2, ConfidencesDropped: 2}, fixes)
	assert.Equal(t, []TranscriptWord{
		{Text: "negativ", Start: 0, End: 0},
		{Text: "baklengs", Start: 1.5, End: 1.5},
		{Text: "grense", Start: 1.6, End: 1.7, Confidence: ptr(1)},
	}, got.Segments[0].Words)
}

func TestNewTranscript_TooManyWordsForOneCue(t *testing.T) {
	var words []TranscriptWord
	for range MaxWordsPerCue + 1 {
		words = append(words, TranscriptWord{Text: "ord", Start: 3.5, End: 3.6})
	}
	words = append(words, TranscriptWord{Text: "slutt", Start: 6, End: 7})

	got, fixes := NewTranscript(cues, words)

	assert.Equal(t, WordFixes{WordsDropped: MaxWordsPerCue + 1}, fixes)
	assert.Nil(t, got.Segments[1].Words)
	assert.Len(t, got.Segments[2].Words, 1, "other cues keep their words")
}

func TestNewTranscript_ExactlyMaxWords(t *testing.T) {
	var words []TranscriptWord
	for range MaxWordsPerCue {
		words = append(words, TranscriptWord{Text: "ord", Start: 3.5, End: 3.6})
	}
	got, fixes := NewTranscript(cues, words)
	assert.Zero(t, fixes)
	assert.Len(t, got.Segments[1].Words, MaxWordsPerCue)
}

func TestNewTranscript_NoCues(t *testing.T) {
	got, _ := NewTranscript(nil, []TranscriptWord{{Text: "Hei", Start: 1, End: 2}})
	assert.Empty(t, got.Segments)
}

func TestTranscript_JSON(t *testing.T) {
	tr, _ := NewTranscript(cues[:1], []TranscriptWord{
		{Text: "Hei", Start: 1, End: 1.4, Confidence: ptr(0.93)},
		{Text: "du", Start: 1.5, End: 1.8},
	})
	b, err := json.Marshal(tr)
	require.NoError(t, err)
	assert.JSONEq(t, `{"segments": [{"start": 1, "end": 2, "text": "Hei", "words": [
		{"text": "Hei", "start": 1, "end": 1.4, "confidence": 0.93},
		{"text": "du", "start": 1.5, "end": 1.8}
	]}]}`, string(b))

	b, err = json.Marshal(tr.WithoutWords())
	require.NoError(t, err)
	assert.JSONEq(t, `{"segments": [{"start": 1, "end": 2, "text": "Hei"}]}`, string(b))
	assert.Len(t, tr.Segments[0].Words, 2, "WithoutWords leaves the transcript as it was")
}
