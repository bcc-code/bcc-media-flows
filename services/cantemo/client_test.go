package cantemo

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

// TestAddRelation tests is a bare-bones check,
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

func TestClient_GetTranscriptionJSON(t *testing.T) {
	baseURL := os.Getenv("CANTEMO_URL")
	token := os.Getenv("CANTEMO_TOKEN")

	if baseURL == "" {
		t.Skip("CANTEMO_URL not set")
	}

	if token == "" {
		t.Skip("CANTEMO_TOKEN not set")
	}

	client := NewClient(baseURL, token)
	tr, err := client.GetTranscriptionJSON("VX-486350")
	assert.NoError(t, err)
	assert.NotEmpty(t, tr)
}

func TestClient_GetPreview(t *testing.T) {
	baseURL := os.Getenv("CANTEMO_URL")
	token := os.Getenv("CANTEMO_TOKEN")

	if baseURL == "" {
		t.Skip("CANTEMO_URL not set")
	}

	if token == "" {
		t.Skip("CANTEMO_TOKEN not set")
	}

	client := NewClient(baseURL, token)
	meta, err := client.GetPreviewUrl("VX-486350")
	assert.NoError(t, err)
	assert.NotEmpty(t, meta)
}
