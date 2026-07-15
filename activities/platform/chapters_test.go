package platform_activities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSongExtract(t *testing.T) {
	tests := []struct {
		label      string
		collection string
		number     string
	}{
		{"NHV-123", "NHV", "123"},
		{"NHV 123", "NHV", "123"},
		{"NHV123", "NHV", "123"},
		{"HV-123", "HV", "123"},
		{"HV 12", "HV", "12"},
		{"HV12", "HV", "12"},
		{"HV - 12", "HV", "12"},
		{"FMB 45", "FMB", "45"},
		{"FMB-45", "FMB", "45"},
		{"SONG TITLE - NHV 7", "NHV", "7"},
		{"SONG TITLE - HV 7", "HV", "7"},
		{"NO SONG REFERENCE", "", ""},
		{"ARCHVAULT 12", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			match := SongExtract.FindStringSubmatch(tt.label)
			if tt.collection == "" {
				assert.Nil(t, match)
				return
			}
			assert.Len(t, match, 3)
			assert.Equal(t, tt.collection, match[1])
			assert.Equal(t, tt.number, match[2])
		})
	}
}
