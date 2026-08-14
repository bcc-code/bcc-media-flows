package export

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func twoClipExportData() *vidispine.ExportData {
	return &vidispine.ExportData{
		SafeTitle: "Some_Title",
		Clips: []*vidispine.Clip{
			{
				VideoFile:  "/mnt/isilon/a.mxf",
				InSeconds:  0,
				OutSeconds: 10,
				AudioFiles: map[string]*vidispine.AudioFile{
					"nor": {File: "/mnt/isilon/a-nor.wav"},
					"eng": {File: "/mnt/isilon/a-eng.wav"},
					"deu": {File: "/mnt/isilon/a-deu.wav"},
				},
				SubtitleFiles: map[string]string{
					"nor": "/mnt/isilon/a-nor.srt",
					"eng": "/mnt/isilon/a-eng.srt",
				},
				JSONTranscriptFile: "/mnt/isilon/a.json",
			},
			{
				VideoFile:  "/mnt/isilon/b.mxf",
				InSeconds:  5,
				OutSeconds: 20,
				AudioFiles: map[string]*vidispine.AudioFile{
					"nor": {File: "/mnt/isilon/b-nor.wav"},
					"eng": {File: "/mnt/isilon/b-eng.wav"},
					"deu": {File: "/mnt/isilon/b-deu.wav"},
				},
				SubtitleFiles: map[string]string{
					"nor": "/mnt/isilon/b-nor.srt",
					"eng": "/mnt/isilon/b-eng.srt",
				},
			},
		},
	}
}

// The result is used directly in workflow code now rather than being frozen by
// a SideEffect marker, so it has to be the same on every call — including
// across the randomized map iteration inside.
func TestExportDataToMergeInputsIsStableAcrossCalls(t *testing.T) {
	tempDir := paths.MustParse("/mnt/isilon/temp")
	subsDir := paths.MustParse("/mnt/isilon/subs")

	want := exportDataToMergeInputs(twoClipExportData(), tempDir, subsDir)

	for i := 0; i < 50; i++ {
		assert.Equal(t, want, exportDataToMergeInputs(twoClipExportData(), tempDir, subsDir))
	}
}

func TestExportDataToMergeInputsBuildsOneInputPerLanguage(t *testing.T) {
	tempDir := paths.MustParse("/mnt/isilon/temp")
	subsDir := paths.MustParse("/mnt/isilon/subs")

	got := exportDataToMergeInputs(twoClipExportData(), tempDir, subsDir)

	require.Len(t, got.AudioMergeInputs, 3)
	require.Len(t, got.SubtitleMergeInputs, 2)

	// Clip order drives item order within a language; the map iteration must not
	// be able to reorder it.
	nor := got.AudioMergeInputs["nor"]
	require.Len(t, nor.Items, 2)
	assert.Equal(t, paths.MustParse("/mnt/isilon/a-nor.wav"), nor.Items[0].Path)
	assert.Equal(t, paths.MustParse("/mnt/isilon/b-nor.wav"), nor.Items[1].Path)
	assert.Equal(t, "Some_Title-nor", nor.Title)

	assert.Equal(t, subsDir, got.SubtitleMergeInputs["eng"].OutputDir)
	assert.Equal(t, tempDir, got.SubtitleMergeInputs["eng"].WorkDir)

	// 10 + 15 seconds of video, and only the first clip has a transcript.
	assert.Equal(t, float64(25), got.MergeInput.Duration)
	require.NotNil(t, got.JSONTranscriptInput)
	assert.Equal(t, float64(10), got.JSONTranscriptInput.Duration)
}

func TestExportDataToMergeInputsHasNoTranscriptWhenNoClipHasOne(t *testing.T) {
	data := twoClipExportData()
	data.Clips[0].JSONTranscriptFile = ""

	got := exportDataToMergeInputs(data, paths.MustParse("/mnt/isilon/temp"), paths.MustParse("/mnt/isilon/subs"))

	assert.Nil(t, got.JSONTranscriptInput)
}
