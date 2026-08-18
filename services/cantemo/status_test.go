package cantemo

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cantemoServer serves one fixed response for every request.
func cantemoServer(t *testing.T, status int, contentType, body string) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return NewClient(testConfig{url: server.URL, token: "token"})
}

// Hole 1, the important one. resty only unmarshals an error body for JSON/XML, so the
// old hook saw a nil resp.Error() for an HTML response and returned nil. Callers then
// got a zero-valued result with no error — for GetFiles that meant
// MoveFileByImportDate iterating zero objects and reporting success.
func TestClient_HTMLErrorBodyIsAnError(t *testing.T) {
	client := cantemoServer(t, http.StatusBadGateway, "text/html", "<html>502 Bad Gateway</html>")

	result, err := client.GetFiles("/mnt/x", "imported", "storage", 0, "")

	require.Error(t, err, "an HTML 502 must not read as a file listing with no files")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "502")
}

// Same hole with an empty body, which is what a bare 404 from a gateway looks like.
func TestClient_EmptyErrorBodyIsAnError(t *testing.T) {
	client := cantemoServer(t, http.StatusNotFound, "text/plain", "")

	result, err := client.GetFiles("/mnt/x", "imported", "storage", 0, "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "404")
}

// Hole 2: a JSON error envelope with no "detail" key produced merry.New(""), a non-nil
// error carrying no message at all, which then surfaced in Temporal as a blank failure.
func TestClient_JSONErrorWithoutDetailStillDescribesItself(t *testing.T) {
	client := cantemoServer(t, http.StatusInternalServerError, "application/json", `{"other":"field"}`)

	_, err := client.GetFiles("/mnt/x", "imported", "storage", 0, "")

	require.Error(t, err)
	assert.NotEmpty(t, strings.TrimSpace(err.Error()), "the error must say something")
	assert.Contains(t, err.Error(), "500")
}

// The structured envelope is still preferred when present.
func TestClient_JSONDetailIsUsed(t *testing.T) {
	client := cantemoServer(t, http.StatusForbidden, "application/json",
		`{"detail":"You do not have permission to perform this action."}`)

	_, err := client.GetFiles("/mnt/x", "imported", "storage", 0, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "You do not have permission")
	assert.Contains(t, err.Error(), "403")
}

// AddRelation returns only an error, so there is no result to inspect. A swallowed
// failure let import_audio_from_reaper write RelatedMBFieldID metadata as though the
// relation existed.
func TestClient_AddRelationReportsFailure(t *testing.T) {
	client := cantemoServer(t, http.StatusBadGateway, "text/html", "<html>nope</html>")

	err := client.AddRelation("VX-1", "VX-2")

	require.Error(t, err, "a failed relation must not look like a created one")
}

// GetLookupChoices returning an empty map with a nil error made every chapter publish
// its raw machine key instead of the human label.
func TestClient_GetLookupChoicesReportsFailure(t *testing.T) {
	client := cantemoServer(t, http.StatusInternalServerError, "text/html", "<html>boom</html>")

	choices, err := client.GetLookupChoices("group", "field")

	require.Error(t, err, "an empty choice map must not be indistinguishable from a failure")
	assert.Nil(t, choices)
}

// A large error body must not be quoted in full: these errors are stored in the
// Temporal workflow history.
func TestClient_ErrorBodyIsTruncated(t *testing.T) {
	client := cantemoServer(t, http.StatusInternalServerError, "text/html",
		"<html>"+strings.Repeat("x", 50_000)+"</html>")

	_, err := client.GetFiles("/mnt/x", "imported", "storage", 0, "")

	require.Error(t, err)
	assert.Less(t, len(err.Error()), 2048, "error body should be bounded")
	assert.Contains(t, err.Error(), "truncated")
}

// Success must be untouched by the hook, including the timestamp parsing that follows.
func TestClient_SuccessIsUnaffected(t *testing.T) {
	client := cantemoServer(t, http.StatusOK, "application/json",
		`{"objects":[{"name":"a.mxf","timestamp":"2021-04-20T16:44:51.790+0000"}]}`)

	result, err := client.GetFiles("/mnt/x", "imported", "storage", 0, "")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Objects, 1)
}

// A genuinely empty listing on a 200 is still an empty listing, not an error — the
// distinction the hook exists to restore.
func TestClient_EmptySuccessIsNotAnError(t *testing.T) {
	client := cantemoServer(t, http.StatusOK, "application/json", `{"objects":[]}`)

	result, err := client.GetFiles("/mnt/x", "imported", "storage", 0, "")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Objects)
}
