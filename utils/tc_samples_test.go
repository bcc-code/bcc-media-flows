package utils_test

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/stretchr/testify/assert"
)

func TestTCToSamples(t *testing.T) {
	type args struct {
		tc         string
		fps        int
		sampleRate int
		expected   int
		expectErr  bool
	}

	tests := []args{
		{"0:00:01:00", 25, 48000, 48000, false},
		{"0:00:00:01", 25, 48000, 1920, false},
		{"13:50:38:05", 25, 48000, 2392233600, false},
		{"180000@PAL", 25, 48000, 345600000, false},
		{"180000@NTSC", 0, 48000, 288288000, false},
		{"0:00:01:00", 0, 48000, 0, true},
		{"0:00:01:00", -25, 48000, 0, true},
	}

	for _, tt := range tests {
		res, err := utils.TCToSamples(tt.tc, tt.fps, tt.sampleRate)
		if tt.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
		assert.Equal(t, tt.expected, res)
	}
}
