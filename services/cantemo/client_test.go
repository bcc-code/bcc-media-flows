package cantemo

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

// TestAddRelation tests is a bare bones check,
// it does not check if the relation was actually added.
func TestAddRelation(t *testing.T) {
	cantemoBaseURL := os.Getenv("CANTEMO_URL")
	cantemoToken := os.Getenv("CANTEMO_TOKEN")

	if cantemoBaseURL == "" {
		t.Skip("CANTEMO_URL not set")
	}

	if cantemoToken == "" {
		t.Skip("CANTEMO_TOKEN not set")
	}

	client := NewClient(cantemoBaseURL, cantemoToken)
	err := client.AddRelation("VX-parent", "VX-child")
	assert.NoError(t, err)
}
