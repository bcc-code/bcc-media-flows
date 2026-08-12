package transcribe

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testdataPath(name string) paths.Path {
	return paths.Path{Drive: paths.TestDrive, Path: name}
}

func mergeInput(items ...common.MergeInputItem) common.MergeInput {
	return common.MergeInput{Title: "test", Items: items}
}

// An unreadable input must be reported rather than skipped. Dropping one cut still
// produces a plausible-looking transcript, which the caller writes to disk as if it
// were complete — so the failure has to travel out through the signature.
func TestMergeTranscripts_UnreadableFileIsReported(t *testing.T) {
	result, err := MergeTranscripts(mergeInput(
		common.MergeInputItem{Path: testdataPath("does_not_exist.json"), Start: 0, End: 10},
	))

	require.Error(t, err, "an unreadable transcript must not be silently skipped")
	assert.Nil(t, result)
}

// Malformed JSON takes the same path, since JsonFileToStruct returns the unmarshal
// error.
func TestMergeTranscripts_MalformedJSONIsReported(t *testing.T) {
	result, err := MergeTranscripts(mergeInput(
		common.MergeInputItem{Path: testdataPath("malformed.json"), Start: 0, End: 10},
	))

	require.Error(t, err)
	assert.Nil(t, result)
}

// A partial failure is the dangerous case: one good cut and one bad one must report
// the error, not return a plausible-looking transcript containing only the good half.
func TestMergeTranscripts_PartialFailureIsReported(t *testing.T) {
	result, err := MergeTranscripts(mergeInput(
		common.MergeInputItem{Path: testdataPath("transcript_a.json"), Start: 0, End: 10},
		common.MergeInputItem{Path: testdataPath("does_not_exist.json"), Start: 0, End: 10},
	))

	require.Error(t, err, "a merged transcript missing a cut must not be returned as success")
	assert.Nil(t, result)
}

// The happy path still merges, so the change is not just failing everything.
func TestMergeTranscripts_MergesReadableInput(t *testing.T) {
	result, err := MergeTranscripts(mergeInput(
		common.MergeInputItem{Path: testdataPath("transcript_a.json"), Start: 0, End: 10},
	))

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Segments, 2)
	assert.Contains(t, result.Text, "hello")
	assert.Contains(t, result.Text, "world")
	assert.Equal(t, "no", result.Language)
}

// Two readable cuts are offset by the duration of the ones before them.
func TestMergeTranscripts_OffsetsLaterCuts(t *testing.T) {
	result, err := MergeTranscripts(mergeInput(
		common.MergeInputItem{Path: testdataPath("transcript_a.json"), Start: 0, End: 4},
		common.MergeInputItem{Path: testdataPath("transcript_a.json"), Start: 0, End: 4},
	))

	require.NoError(t, err)
	require.Len(t, result.Segments, 4)

	// First cut keeps its own timings; the second is pushed out by the first's length.
	assert.Equal(t, 0.0, result.Segments[0].Start)
	assert.Equal(t, 4.0, result.Segments[2].Start)
}

func TestMergeTranscripts_NoItems(t *testing.T) {
	result, err := MergeTranscripts(mergeInput())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Segments)
}
