package vidispine_test

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"

	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/stretchr/testify/assert"
)

// Just make sure decoding works
func Test_DecodeSequenceXML(t *testing.T) {
	glob, err := filepath.Glob("testdata/sequences/*.xml")
	assert.NoError(t, err)

	for _, file := range glob {
		f, err := os.ReadFile(file)
		assert.NoError(t, err)

		doc := &vidispine.SequenceDocument{}
		err = xml.Unmarshal(f, doc)
		assert.NoError(t, err)
	}
}

func Test_Convert(t *testing.T) {

	clip := &vidispine.Clip{
		InSeconds:   10,
		OutSeconds:  20,
		SequenceIn:  30,
		SequenceOut: 40,
	}

	chapter := &vidispine.MetadataResult{
		Terse: map[string][]*vidispine.MetadataField{
			"title": []*vidispine.MetadataField{
				&vidispine.MetadataField{
					Start: "250@PAL",
					End:   "300@PAL",
					UUID:  "uuid1",
					Value: "chapter1",
				},
			},
		},
	}

	tcStart := 0.0

	expectd := &vidispine.MetadataResult{
		Terse: map[string][]*vidispine.MetadataField{
			"title": []*vidispine.MetadataField{
				&vidispine.MetadataField{
					Start: "750@PAL",
					End:   "800@PAL",
					UUID:  "uuid1",
					Value: "chapter1",
				},
			},
		},
	}

	out := vidispine.ConvertFromClipTCTimeSpaceToSequenceRelativeTimeSpace(clip, chapter, tcStart)
	assert.Equal(t, expectd, out)
}

func Test_Convert2(t *testing.T) {
	clip := &vidispine.Clip{
		InSeconds:   10,
		OutSeconds:  20,
		SequenceIn:  30,
		SequenceOut: 40,
	}

	chapter := &vidispine.MetadataResult{
		Terse: map[string][]*vidispine.MetadataField{
			"title": []*vidispine.MetadataField{
				&vidispine.MetadataField{
					Start: "25250@PAL",
					End:   "25300@PAL",
					UUID:  "uuid1",
					Value: "chapter1",
				},
			},
		},
	}

	tcStart := 1000.0

	expectd := &vidispine.MetadataResult{
		Terse: map[string][]*vidispine.MetadataField{
			"title": []*vidispine.MetadataField{
				&vidispine.MetadataField{
					Start: "750@PAL",
					End:   "800@PAL",
					UUID:  "uuid1",
					Value: "chapter1",
				},
			},
		},
	}

	out := vidispine.ConvertFromClipTCTimeSpaceToSequenceRelativeTimeSpace(clip, chapter, tcStart)
	assert.Equal(t, expectd, out)
}
