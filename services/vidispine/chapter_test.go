package vidispine

import (
	"os"
	"testing"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/stretchr/testify/assert"
)

func TestMergeTerseTimecodes(t *testing.T) {
	terseA := map[string][]*vsapi.MetadataField{
		"title": {
			{
				Start: "10@PAL",
				End:   "20@PAL",
			},
		},
	}

	terseB := map[string][]*vsapi.MetadataField{
		"title": {
			{
				Start: "15@PAL",
				End:   "25@PAL",
			},
		},
		"something_else": {
			{
				Start: "15@PAL",
				End:   "25@PAL",
			},
		},
	}

	expected := map[string][]*vsapi.MetadataField{
		"title": {
			{
				Start: "10@PAL",
				End:   "25@PAL",
			},
		},
		"something_else": {
			{
				Start: "10@PAL",
				End:   "25@PAL",
			},
		},
	}

	result := mergeTerseTimecodes(terseA, terseB)

	assert.Equal(t, expected, result)
}

// This is an edge case where the annotations overlap by a few frames
func Test_GetChapterMetaForClips_Overlapping(t *testing.T) {
	if os.Getenv("VIDISPINE_BASE_URL") == "" {
		t.Skip("VIDISPINE_BASE_URL is not set")
	}

	// The important thing here is the VXID of the main file (not the audio file),
	// together with the in and out points of the clip.
	//
	// The situation is that two annotations (chapters) in MB overlap by a few frames,
	// making the system think that there are two chapters between the in and out point.
	// While this is techinically correct, it is not what we want, if one of the "chapters"
	// is very short (less than 10 seconds). At this point it is unlikely to be a real chapter and we can ignore it.
	testData := [][]*Clip{
		{
			{
				VideoFile:   "/dummy/file.mp4",
				InSeconds:   1420.1199999999953,
				OutSeconds:  2767.439999999988,
				SequenceIn:  0,
				SequenceOut: 0,
				AudioFiles: map[string]*AudioFile{
					"nor": {
						VXID:    "VX-489605",
						Streams: []int{2},
						File:    "/dummy/file.wav",
					},
				},
				SubtitleFiles:      map[string]string{},
				JSONTranscriptFile: "",
				VXID:               "VX-489598",
			},
		},
		{
			{
				VideoFile:   "/dummy/file.mp4",
				InSeconds:   3750.9199999999983,
				OutSeconds:  3906.87999999999,
				SequenceIn:  0,
				SequenceOut: 0,
				AudioFiles: map[string]*AudioFile{
					"nor": {
						VXID:    "VX-489605",
						Streams: []int{2},
						File:    "/dummy/file.wav",
					},
				},
				SubtitleFiles:      map[string]string{},
				JSONTranscriptFile: "",
				VXID:               "VX-489598",
			},
		},
	}

	for _, clips := range testData {

		client := vsapi.NewClient(
			os.Getenv("VIDISPINE_BASE_URL"),
			os.Getenv("VIDISPINE_USERNAME"),
			os.Getenv("VIDISPINE_PASSWORD"),
		)
		out, err := GetChapterMetaForClips(client, clips)
		assert.NoError(t, err)
		assert.NotEmpty(t, out)
		assert.Len(t, out, 1)

		for i, chapter := range out {
			assert.GreaterOrEqual(t, len(chapter.Meta.Terse["title"]), 1, "Chapter in loop iteration %d has no title", i)
		}
	}
}
