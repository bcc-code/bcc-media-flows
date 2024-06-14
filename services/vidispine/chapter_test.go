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

func Test_GetChapterMetaForClips(t *testing.T) {
	clips := []*Clip{
		{
			VideoFile:   "/dummy/file.mp4",
			InSeconds:   1907.7599999999948,
			OutSeconds:  3025.399999999994,
			SequenceIn:  0,
			SequenceOut: 0,
			AudioFiles: map[string]*AudioFile{
				"nor": {
					VXID:    "VX-486737",
					Streams: []int{1, 2},
					File:    "/dummy/file.wav",
				},
			},
			SubtitleFiles:      map[string]string{},
			JSONTranscriptFile: "",
			VXID:               "VX-486737",
		},
	}

	client := vsapi.NewClient(
		os.Getenv("VIDISPINE_BASE_URL"),
		os.Getenv("VIDISPINE_USERNAME"),
		os.Getenv("VIDISPINE_PASSWORD"),
	)
	out, err := GetChapterMetaForClips(client, clips)
	assert.NoError(t, err)
	assert.NotEmpty(t, out)

	for i, chapter := range out {
		assert.GreaterOrEqual(t, len(chapter.Meta.Terse["title"]), 1, "Chapter in loop iteration %d has no title", i)
	}

}
