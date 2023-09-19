package vidispine

import (
	"testing"

	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
	"github.com/stretchr/testify/assert"
)

func Test_Convert(t *testing.T) {

	clip := &Clip{
		InSeconds:   10,
		OutSeconds:  20,
		SequenceIn:  30,
		SequenceOut: 40,
	}

	chapter := &vsapi.MetadataResult{
		Terse: map[string][]*vsapi.MetadataField{
			"title": {
				{
					Start: "250@PAL",
					End:   "300@PAL",
					UUID:  "uuid1",
					Value: "chapter1",
				},
			},
		},
	}

	tcStart := 0.0

	expectd := &vsapi.MetadataResult{
		Terse: map[string][]*vsapi.MetadataField{
			"title": {
				{
					Start: "750@PAL",
					End:   "800@PAL",
					UUID:  "uuid1",
					Value: "chapter1",
				},
			},
		},
	}

	out := convertFromClipTCTimeToSequenceRelativeTime(clip, chapter, tcStart)
	assert.Equal(t, expectd, out)
}

func Test_Convert2(t *testing.T) {
	clip := &Clip{
		InSeconds:   10,
		OutSeconds:  20,
		SequenceIn:  30,
		SequenceOut: 40,
	}

	chapter := &vsapi.MetadataResult{
		Terse: map[string][]*vsapi.MetadataField{
			"title": {
				{
					Start: "25250@PAL",
					End:   "25300@PAL",
					UUID:  "uuid1",
					Value: "chapter1",
				},
			},
		},
	}

	tcStart := 1000.0

	expectd := &vsapi.MetadataResult{
		Terse: map[string][]*vsapi.MetadataField{
			"title": {
				{
					Start: "750@PAL",
					End:   "800@PAL",
					UUID:  "uuid1",
					Value: "chapter1",
				},
			},
		},
	}

	out := convertFromClipTCTimeToSequenceRelativeTime(clip, chapter, tcStart)
	assert.Equal(t, expectd, out)
}
