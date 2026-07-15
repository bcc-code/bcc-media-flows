package bmm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSongbookPrefix(t *testing.T) {
	tests := []struct {
		slug     string
		expected string
	}{
		// Legacy songbooks stored as lowercase slugs in BMM.
		{"herrens_veier", "HV"},
		{"mandelblomsten", "FMB"},
		// Newer songbooks store the uppercase abbreviation directly as the rel name.
		{"NHV", "NHV"},
		{"NFMB", "NFMB"},
		{"RB", "RB"},
		{"SOS", "SOS"},
		// Unknown lowercase slugs fall back to the initials heuristic.
		{"some_new_book", "SNB"},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			assert.Equal(t, tt.expected, songbookPrefix(tt.slug))
		})
	}
}
