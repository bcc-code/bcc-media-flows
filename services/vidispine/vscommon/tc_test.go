package vscommon_test

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/stretchr/testify/assert"
)

func Test_TCToSeconds(t *testing.T) {
	testData := []struct {
		in    string
		out   float64
		isErr bool
	}{
		{"0@PAL", 0, false},
		{"25@PAL", 1.0, false},
		{"25000@PAL", 1000.0, false},
		{"25000@NTSC", 0.0, true},
	}

	for _, td := range testData {
		out, err := vscommon.TCToSeconds(td.in)
		assert.Equal(t, td.out, out)
		assert.Equal(t, td.isErr, err != nil)
	}
}
