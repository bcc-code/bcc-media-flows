package activities

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/mediabrain"
)

type brainConfig struct{ base string }

func (c brainConfig) BaseURL() string  { return c.base }
func (c brainConfig) TokenURL() string { return c.base + "/token" }
func (c brainConfig) ClientID() string { return "client-id" }
func (c brainConfig) Username() string { return "flows" }
func (c brainConfig) Password() string { return "app-password" }

// brainServer finds asset a1 with version v1 and records the importSubtitles input
func brainServer(t *testing.T) (*mediabrain.Client, *map[string]any) {
	t.Helper()
	imported := map[string]any{}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"jwt","token_type":"Bearer","expires_in":300}`))
	})
	mux.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Variables map[string]any `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		if input, ok := body.Variables["input"].(map[string]any); ok {
			imported = input
			_, _ = w.Write([]byte(`{"data":{"importSubtitles":{"id":"t1","assetId":"a1","renditionId":"r1","revision":1}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"assets":{"nodes":[{"id":"a1","versions":{"nodes":[{"id":"v1","number":1}]}}]}}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return mediabrain.NewClient(brainConfig{base: srv.URL}), &imported
}

func writeSRT(t *testing.T) paths.Path {
	t.Helper()
	file := filepath.Join(t.TempDir(), "audio.srt")
	require.NoError(t, os.WriteFile(file, []byte("1\n00:00:00,000 --> 00:00:01,000\nHei\n"), 0o644))
	return paths.MustParse(file)
}

func TestImportTranscriptToMediaBrain_SendsASRAndModel(t *testing.T) {
	client, imported := brainServer(t)
	a := &MediaBrainActivities{Client: client}
	env := activityEnv(t)
	env.RegisterActivity(a.ImportTranscriptToMediaBrain)

	_, err := env.ExecuteActivity(a.ImportTranscriptToMediaBrain, ImportTranscriptToMediaBrainParams{
		VXID: "VX-1", Language: "no", SRTPath: writeSRT(t), Model: "nb-whisper-large",
	})
	require.NoError(t, err)

	assert.Equal(t, "ASR", (*imported)["origin"])
	assert.Equal(t, "nb-whisper-large", (*imported)["model"])
	assert.Equal(t, "TRANSCRIPT", (*imported)["kind"])
}

func TestImportTranscriptToMediaBrain_NoModel(t *testing.T) {
	client, imported := brainServer(t)
	a := &MediaBrainActivities{Client: client}
	env := activityEnv(t)
	env.RegisterActivity(a.ImportTranscriptToMediaBrain)

	_, err := env.ExecuteActivity(a.ImportTranscriptToMediaBrain, ImportTranscriptToMediaBrainParams{
		VXID: "VX-1", Language: "no", SRTPath: writeSRT(t), Model: "  ",
	})
	require.NoError(t, err)

	assert.Equal(t, "ASR", (*imported)["origin"])
	assert.NotContains(t, *imported, "model")
}
