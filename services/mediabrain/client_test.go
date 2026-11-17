package mediabrain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bcc-code/bcc-media-flows/internal/httpx"
)

type testConfig struct{ base, token string }

func (c testConfig) BaseURL() string  { return c.base }
func (c testConfig) TokenURL() string { return c.token }
func (c testConfig) ClientID() string { return "client-id" }
func (c testConfig) Username() string { return "flows" }
func (c testConfig) Password() string { return "app-password" }

type recorded struct {
	auth      string
	query     string
	variables map[string]any
}

// server answers the token endpoint and /query; reply writes the GraphQL answer.
func server(t *testing.T, reply func(w http.ResponseWriter)) (*Client, *recorded) {
	t.Helper()
	rec := &recorded{}
	tokenRequests := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		tokenRequests++
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))
		assert.Equal(t, "client-id", r.Form.Get("client_id"))
		assert.Equal(t, "flows", r.Form.Get("username"))
		assert.Equal(t, "app-password", r.Form.Get("password"))
		assert.Equal(t, "openid profile", r.Form.Get("scope"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"jwt-1","token_type":"Bearer","expires_in":300}`))
	})
	mux.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		rec.auth = r.Header.Get("Authorization")
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		rec.query, rec.variables = body.Query, body.Variables
		w.Header().Set("Content-Type", "application/json")
		reply(w)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Cleanup(func() { assert.LessOrEqual(t, tokenRequests, 1, "the token should be reused") })
	return NewClient(testConfig{base: srv.URL, token: srv.URL + "/token"}), rec
}

func TestFindAssetByLegacyID(t *testing.T) {
	c, rec := server(t, func(w http.ResponseWriter) {
		_, _ = w.Write([]byte(`{"data":{"assets":{"nodes":[{"id":"a1","title":"T","versions":{"nodes":[{"id":"v1","number":1},{"id":"v3","number":3},{"id":"v2","number":2}]}}]}}}`))
	})

	asset, err := c.FindAssetByLegacyID(context.Background(), "VX-123")
	require.NoError(t, err)
	assert.Equal(t, "Bearer jwt-1", rec.auth)
	assert.Equal(t, map[string]any{"legacyId": "VX-123"}, rec.variables)
	assert.Equal(t, "a1", asset.ID)

	latest, err := asset.LatestVersion()
	require.NoError(t, err)
	assert.Equal(t, "v3", latest.ID)

	// A second call reuses the token.
	_, err = c.FindAssetByLegacyID(context.Background(), "VX-123")
	require.NoError(t, err)
}

func TestFindAssetByLegacyID_NoneOrMany(t *testing.T) {
	c, _ := server(t, func(w http.ResponseWriter) {
		_, _ = w.Write([]byte(`{"data":{"assets":{"nodes":[]}}}`))
	})
	_, err := c.FindAssetByLegacyID(context.Background(), "VX-1")
	assert.ErrorIs(t, err, ErrAssetNotFound)

	c, _ = server(t, func(w http.ResponseWriter) {
		_, _ = w.Write([]byte(`{"data":{"assets":{"nodes":[{"id":"a1"},{"id":"a2"}]}}}`))
	})
	_, err = c.FindAssetByLegacyID(context.Background(), "VX-1")
	assert.ErrorIs(t, err, ErrAmbiguousAsset)
}

func TestLatestVersion_NoVersions(t *testing.T) {
	_, err := Asset{ID: "a1"}.LatestVersion()
	assert.ErrorIs(t, err, ErrNoVersion)
}

func TestImportSubtitles(t *testing.T) {
	c, rec := server(t, func(w http.ResponseWriter) {
		_, _ = w.Write([]byte(`{"data":{"importSubtitles":{"id":"t1","assetId":"a1","renditionId":"r1","revision":2}}}`))
	})

	lang := "no"
	track, err := c.ImportSubtitles(context.Background(), ImportSubtitlesInput{
		VersionID: "v1", Language: &lang, Kind: TrackKindTranscript, Format: SubtitleFormatSRT, Content: "1\n",
	})
	require.NoError(t, err)
	assert.Equal(t, &SubtitleTrack{ID: "t1", AssetID: "a1", RenditionID: "r1", Revision: 2}, track)
	assert.Equal(t, map[string]any{"input": map[string]any{
		"versionId": "v1", "language": "no", "kind": "TRANSCRIPT", "format": "SRT", "content": "1\n",
	}}, rec.variables)
}

func TestImportSubtitles_NoLanguage(t *testing.T) {
	c, rec := server(t, func(w http.ResponseWriter) {
		_, _ = w.Write([]byte(`{"data":{"importSubtitles":{"id":"t1"}}}`))
	})
	_, err := c.ImportSubtitles(context.Background(), ImportSubtitlesInput{
		VersionID: "v1", Kind: TrackKindTranscript, Format: SubtitleFormatSRT, Content: "1\n",
	})
	require.NoError(t, err)
	assert.NotContains(t, rec.variables["input"], "language")
}

func TestGraphQLErrorWith200(t *testing.T) {
	c, _ := server(t, func(w http.ResponseWriter) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"content is too large","extensions":{"code":"INVALID_INPUT"}}],"data":null}`))
	})
	_, err := c.ImportSubtitles(context.Background(), ImportSubtitlesInput{VersionID: "v1"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInput)
	assert.Contains(t, err.Error(), "content is too large")
}

func TestRefusedByAuthMiddleware(t *testing.T) {
	c, _ := server(t, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"extensions":{"code":"FORBIDDEN"},"message":"forbidden"}]}`))
	})
	_, err := c.FindAssetByLegacyID(context.Background(), "VX-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrForbidden)
	assert.Equal(t, http.StatusForbidden, httpx.StatusCode(err))
}
