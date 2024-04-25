package ingestworkflows

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type duplicatePathTestData struct {
	input    string
	expected string
}

func TestSanizizeDuplicatePaths(t *testing.T) {

	data := []duplicatePathTestData{
		{"1/2/3/4", "1/2/3/4"},
		{"1/2/3/4/4/3/2/1", "1/2/3/4/4/3/2/1"},
		{"/1/2/1/2//", "/1/2"},
	}

	for _, d := range data {
		result := sanitizeDuplicatdPath(d.input)
		assert.Equal(t, d.expected, result)
	}
}
