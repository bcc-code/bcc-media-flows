package vidispine

import (
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
