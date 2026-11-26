package transcribe

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The transcriber writes no confidence. It must stay missing rather than become 0,
// which would claim the words were certainly wrong.
func TestTranscription_MissingConfidenceStaysMissing(t *testing.T) {
	var tr Transcription
	require.NoError(t, json.Unmarshal([]byte(`{"text": "Hei", "language": "no", "segments": [
		{"id": 0, "start": 1, "end": 2, "text": "Hei", "words": [{"text": "Hei", "start": 1, "end": 1.4}]}
	]}`), &tr))
	assert.Nil(t, tr.Segments[0].Confidence)
	assert.Nil(t, tr.Segments[0].Words[0].Confidence)

	b, err := json.Marshal(tr)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "confidence")
	assert.Contains(t, string(b), `"language":"no"`)
}

func TestTranscription_ConfidenceIsKept(t *testing.T) {
	var tr Transcription
	require.NoError(t, json.Unmarshal([]byte(`{"segments": [
		{"confidence": 0, "words": [{"text": "Hei", "confidence": 0.93}]}
	]}`), &tr))
	require.NotNil(t, tr.Segments[0].Confidence)
	assert.Equal(t, 0.0, *tr.Segments[0].Confidence)
	assert.Equal(t, 0.93, *tr.Segments[0].Words[0].Confidence)
}
