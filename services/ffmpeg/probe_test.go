package ffmpeg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeColorTRCFilter(t *testing.T) {
	cases := []struct {
		name     string
		trc      string
		expected string
	}{
		{"empty returns fix", "", "setparams=color_trc=bt709"},
		{"unknown returns fix", "unknown", "setparams=color_trc=bt709"},
		{"reserved returns fix", "reserved", "setparams=color_trc=bt709"},
		{"reserved with whitespace returns fix", "  reserved  ", "setparams=color_trc=bt709"},
		{"reserved uppercase returns fix", "RESERVED", "setparams=color_trc=bt709"},
		{"bt709 returns empty", "bt709", ""},
		{"smpte170m returns empty", "smpte170m", ""},
		{"bt2020-10 returns empty", "bt2020-10", ""},
		{"smpte2084 returns empty", "smpte2084", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeColorTRCFilter(FFProbeStream{ColorTransfer: tc.trc})
			assert.Equal(t, tc.expected, got)
		})
	}
}
