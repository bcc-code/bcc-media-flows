package activities

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/mediabrain"
)

type brainConfig struct{ base string }

func (c brainConfig) BaseURL() string  { return c.base }
func (c brainConfig) TokenURL() string { return c.base + "/token" }
func (c brainConfig) ClientID() string { return "client-id" }
func (c brainConfig) Username() string { return "flows" }
func (c brainConfig) Password() string { return "app-password" }

const foundAsset = `{"data":{"assets":{"nodes":[{"id":"a1","versions":{"nodes":[{"id":"v1","number":1}]}}]}}}`
const noAsset = `{"data":{"assets":{"nodes":[]}}}`

// fakeBrain answers each operation from its replies, in order, and records what it was sent.
type fakeBrain struct {
	finds    []string
	importVS string

	imported   map[string]any
	vsImported map[string]any
	findCount  int
}

func (b *fakeBrain) client(t *testing.T) *mediabrain.Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"jwt","token_type":"Bearer","expires_in":300}`))
	})
	mux.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(body.Query, "FindAssetByLegacyID"):
			require.Less(t, b.findCount, len(b.finds), "unexpected asset lookup")
			_, _ = w.Write([]byte(b.finds[b.findCount]))
			b.findCount++
		case strings.Contains(body.Query, "AssetTypes"):
			_, _ = w.Write([]byte(`{"data":{"assetTypes":[{"id":2,"name":"Deliverable"},{"id":1,"name":"Raw material"}]}}`))
		case strings.Contains(body.Query, "ImportFromVidispine"):
			require.NotEmpty(t, b.importVS, "unexpected Vidispine import")
			b.vsImported = body.Variables["input"].(map[string]any)
			_, _ = w.Write([]byte(b.importVS))
		case strings.Contains(body.Query, "ImportSubtitles"):
			b.imported = body.Variables["input"].(map[string]any)
			_, _ = w.Write([]byte(`{"data":{"importSubtitles":{"id":"t1","assetId":"a1","renditionId":"r1","revision":1}}}`))
		default:
			t.Fatalf("unexpected query %s", body.Query)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return mediabrain.NewClient(brainConfig{base: srv.URL})
}

// srtFixture is on the test drive: a file in t.TempDir() only parses as a Path when
// the temp dir happens to sit under a drive prefix.
var srtFixture = paths.New(paths.TestDrive, "mediabrain/audio.srt")

func runBrainImport(t *testing.T, brain *fakeBrain, model string) (*ImportTranscriptToMediaBrainResult, error) {
	t.Helper()
	a := &MediaBrainActivities{Client: brain.client(t)}
	env := activityEnv(t)
	env.RegisterActivity(a.ImportTranscriptToMediaBrain)

	val, err := env.ExecuteActivity(a.ImportTranscriptToMediaBrain, ImportTranscriptToMediaBrainParams{
		VXID: "VX-1", Language: "no", SRTPath: srtFixture, Model: model,
	})
	if err != nil {
		return nil, err
	}
	var result ImportTranscriptToMediaBrainResult
	require.NoError(t, val.Get(&result))
	return &result, nil
}

func TestImportTranscriptToMediaBrain_SendsASRAndModel(t *testing.T) {
	brain := &fakeBrain{finds: []string{foundAsset}}
	_, err := runBrainImport(t, brain, "nb-whisper-large")
	require.NoError(t, err)

	assert.Equal(t, "ASR", brain.imported["origin"])
	assert.Equal(t, "nb-whisper-large", brain.imported["model"])
	assert.Equal(t, "TRANSCRIPT", brain.imported["kind"])
	assert.Equal(t, "v1", brain.imported["versionId"])
	assert.Nil(t, brain.vsImported, "an asset Brain has is not imported again")
}

func TestImportTranscriptToMediaBrain_NoModel(t *testing.T) {
	brain := &fakeBrain{finds: []string{foundAsset}}
	_, err := runBrainImport(t, brain, "  ")
	require.NoError(t, err)

	assert.Equal(t, "ASR", brain.imported["origin"])
	assert.NotContains(t, brain.imported, "model")
}

func TestImportTranscriptToMediaBrain_ImportsMissingItem(t *testing.T) {
	brain := &fakeBrain{
		finds:    []string{noAsset},
		importVS: `{"data":{"importFromVidispine":{"id":"a9","versions":{"nodes":[{"id":"v9","number":1}]}}}}`,
	}
	result, err := runBrainImport(t, brain, "")
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"itemId": "VX-1", "typeId": float64(1)}, brain.vsImported, "raw material, no project")
	assert.Equal(t, "v9", brain.imported["versionId"])
	assert.Equal(t, "a9", result.AssetID)
}

// A conflict means the item got imported after the lookup, so the asset is looked up again.
func TestImportTranscriptToMediaBrain_ImportConflictFindsAgain(t *testing.T) {
	brain := &fakeBrain{
		finds:    []string{noAsset, foundAsset},
		importVS: `{"errors":[{"message":"VX-1 is already imported as a1","extensions":{"code":"CONFLICT"}}]}`,
	}
	_, err := runBrainImport(t, brain, "")
	require.NoError(t, err)

	assert.Equal(t, 2, brain.findCount)
	assert.Equal(t, "v1", brain.imported["versionId"])
}

// Only administrators may import, and a retry would be refused the same way.
func TestImportTranscriptToMediaBrain_ImportForbidden(t *testing.T) {
	brain := &fakeBrain{
		finds:    []string{noAsset},
		importVS: `{"errors":[{"message":"forbidden","extensions":{"code":"FORBIDDEN"}}]}`,
	}
	_, err := runBrainImport(t, brain, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mediabrain_refused")
	assert.Nil(t, brain.imported)
}

func runBrainImportFiles(t *testing.T, brain *fakeBrain, srtFile string, jsonFile *paths.Path) mediabrain.Transcript {
	t.Helper()
	a := &MediaBrainActivities{Client: brain.client(t)}
	env := activityEnv(t)
	env.RegisterActivity(a.ImportTranscriptToMediaBrain)

	_, err := env.ExecuteActivity(a.ImportTranscriptToMediaBrain, ImportTranscriptToMediaBrainParams{
		VXID: "VX-1", Language: "no", SRTPath: paths.New(paths.TestDrive, srtFile), JSONPath: jsonFile,
	})
	require.NoError(t, err)

	assert.Equal(t, "JSON", brain.imported["format"])
	var sent mediabrain.Transcript
	require.NoError(t, json.Unmarshal([]byte(brain.imported["content"].(string)), &sent))
	return sent
}

// Whisper writes cues without length; Brain refuses a whole file for one of them.
func TestImportTranscriptToMediaBrain_RepairsCues(t *testing.T) {
	brain := &fakeBrain{finds: []string{foundAsset}}
	sent := runBrainImportFiles(t, brain, "mediabrain/whisper.srt", nil)

	assert.Equal(t, mediabrain.Transcript{Segments: []mediabrain.TranscriptSegment{
		{Start: 1, End: 2, Text: "Hei"},
		{Start: 3, End: 6, Text: "verden"},
		{Start: 6, End: 7, Text: "slutt"},
	}}, sent)
}

// The cues are the SRT's; the JSON only gives them their words.
func TestImportTranscriptToMediaBrain_AttachesWords(t *testing.T) {
	brain := &fakeBrain{finds: []string{foundAsset}}
	jsonFile := paths.New(paths.TestDrive, "mediabrain/whisper.json")
	sent := runBrainImportFiles(t, brain, "mediabrain/whisper.srt", &jsonFile)

	conf := func(f float64) *float64 { return &f }
	assert.Equal(t, mediabrain.Transcript{Segments: []mediabrain.TranscriptSegment{
		{Start: 1, End: 2, Text: "Hei", Words: []mediabrain.TranscriptWord{
			{Text: "Eh,", Start: 0.5, End: 0.8, Confidence: conf(0.41)},
			{Text: "Hei", Start: 1, End: 1.4, Confidence: conf(0.93)},
		}},
		{Start: 3, End: 6, Text: "verden", Words: []mediabrain.TranscriptWord{
			{Text: "verden", Start: 3, End: 3, Confidence: conf(0.87)},
		}},
		{Start: 6, End: 7, Text: "slutt", Words: []mediabrain.TranscriptWord{
			{Text: "slutt", Start: 6, End: 6.6, Confidence: conf(0.99)},
		}},
	}}, sent)
}

// Words are secondary: a JSON file that cannot be used leaves the cues without them.
func TestImportTranscriptToMediaBrain_UnusableJSONImportsWithoutWords(t *testing.T) {
	for _, file := range []string{"mediabrain/broken.json", "mediabrain/missing.json"} {
		t.Run(file, func(t *testing.T) {
			brain := &fakeBrain{finds: []string{foundAsset}}
			jsonFile := paths.New(paths.TestDrive, file)
			sent := runBrainImportFiles(t, brain, "mediabrain/whisper.srt", &jsonFile)

			require.Len(t, sent.Segments, 3)
			for _, s := range sent.Segments {
				assert.Empty(t, s.Words)
			}
		})
	}
}

func TestTranscriptContent_TooLargeDropsWords(t *testing.T) {
	words := make([]mediabrain.TranscriptWord, mediabrain.MaxWordsPerCue)
	for i := range words {
		words[i] = mediabrain.TranscriptWord{Text: strings.Repeat("x", 100), Start: 1, End: 2}
	}
	var transcript mediabrain.Transcript
	for i := range 60 {
		transcript.Segments = append(transcript.Segments, mediabrain.TranscriptSegment{
			Start: float64(i), End: float64(i + 1), Text: "cue", Words: words,
		})
	}

	env := activityEnv(t)
	env.RegisterActivityWithOptions(func(ctx context.Context) (string, error) {
		return transcriptContent(ctx, "VX-1", transcript)
	}, activity.RegisterOptions{Name: "content"})
	val, err := env.ExecuteActivity("content")
	require.NoError(t, err)
	var content string
	require.NoError(t, val.Get(&content))

	var sent mediabrain.Transcript
	require.NoError(t, json.Unmarshal([]byte(content), &sent))
	assert.Equal(t, transcript.WithoutWords(), sent)
}
