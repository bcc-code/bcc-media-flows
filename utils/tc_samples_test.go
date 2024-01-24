package utils_test

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
)

func TestTCToSamples(t *testing.T) {
	type args struct {
		tc          string
		fps         int
		sampleRate  int
		expected    int
		expectedErr error
	}

	tests := []args{
		{"0:00:01:00", 25, 48000, 48000, nil},
		{"0:00:00:01", 25, 48000, 1920, nil},
		{"13:50:38:05", 25, 48000, 2392233600, nil},
	}

	for _, tt := range tests {
		res, err := utils.TCToSamples(tt.tc, tt.fps, tt.sampleRate)
		assert.Equal(t, tt.expected, res)
		assert.Equal(t, tt.expectedErr, err)
	}
}

func TestT(t *testing.T) {
	wavSamples := 2748165413
	mfxSamples := 2641753158
	spew.Dump(wavSamples - mfxSamples)
}
