//go:build integration

// Talks to a live Vidispine instance; see chapter_integration_test.go for why
// these are tag-gated. Run with: go test -tags=integration ./services/vidispine/...
package vsapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetTrash(t *testing.T) {
	if os.Getenv("VIDISPINE_BASE_URL") == "" {
		t.Skip("VIDISPINE_BASE_URL not set")
	}

	if os.Getenv("VIDISPINE_USERNAME") == "" {
		t.Skip("VIDISPINE_USERNAME not set")
	}

	if os.Getenv("VIDISPINE_PASSWORD") == "" {
		t.Skip("VIDISPINE_PASSWORD not set")
	}

	c := NewClient(os.Getenv("VIDISPINE_BASE_URL"), os.Getenv("VIDISPINE_USERNAME"), os.Getenv("VIDISPINE_PASSWORD"))

	res, err := c.GetTrash()
	assert.NoError(t, err)
	assert.NotEmpty(t, res)
}
