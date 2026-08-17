package vsapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bcc-code/bcc-media-flows/internal/httpx"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
)

// vsServer serves a fixed status and body for every request, and records how many
// requests it saw.
func vsServer(t *testing.T, status int, body string) (*Client, *int) {
	t.Helper()

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return NewClient(server.URL, "user", "pass"), &calls
}

// A server error must reach the caller as an error. resty leaves err nil for 4xx/5xx
// and only unmarshals on 2xx, so without the hook the caller receives a pointer to a
// zero-valued struct indistinguishable from a real empty answer.
func TestClient_ServerErrorIsAnError(t *testing.T) {
	client, _ := vsServer(t, http.StatusInternalServerError, `{"internalServer":"boom"}`)

	result, err := client.GetMetadata("VX-1")

	require.Error(t, err, "a 500 must not be reported as success")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "500")
}

// GetMetadata is the site whose silent fallbacks did the most damage: a nil Terse map
// makes every Get(field, fallback) return its fallback, so exports shipped with blank
// audio and wrong timecodes. It must not tolerate 404.
func TestClient_GetMetadataDoesNotTolerate404(t *testing.T) {
	client, _ := vsServer(t, http.StatusNotFound, `{"notFound":{"type":"item","id":"VX-1"}}`)

	result, err := client.GetMetadata("VX-1")

	require.Error(t, err, "a missing item must be an error here, not empty metadata")
	assert.Nil(t, result)
}

// A failed search must not read as an empty result set: utils/workflows/vidispine.go
// treats an empty result as "no item matches", and bmm_track_metadata.go acts on that
// by importing a new track.
func TestClient_SearchFailureIsAnError(t *testing.T) {
	client, _ := vsServer(t, http.StatusBadGateway, `<html>gateway</html>`)

	ids, err := client.SearchByMetadataField("portal_mf1", "value")

	require.Error(t, err, "a failed search must not read as 'no matches'")
	assert.Empty(t, ids)
}

// A non-JSON error body must still produce an error. This is the hole in the cantemo
// client's hook, which relies on resty having parsed an error object.
func TestClient_NonJSONErrorBodyIsStillAnError(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>502 Bad Gateway</html>"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "pass")

	result, err := client.GetShapes("VX-1")

	require.Error(t, err, "an HTML 502 from a proxy must not read as an item with no shapes")
	assert.Nil(t, result)
}

// Shape-tag-not-found has to keep its sentinel through the hook, because
// activities/vidispine/files.go detects it with errors.Is to avoid useless retries.
func TestClient_ShapeTagNotFoundKeepsItsSentinel(t *testing.T) {
	client, _ := vsServer(t, http.StatusNotFound,
		`{"notFound":{"type":"shape-tag","id":"lowres_watermarked"}}`)

	_, err := client.AddShapeToItem("lowres_watermarked", "VX-1", "VX-2")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrShapeTagNotFound),
		"the hook must preserve ErrShapeTagNotFound, got: %v", err)
	assert.Contains(t, err.Error(), "lowres_watermarked")
}

// Other 404s must not masquerade as a missing shape tag.
func TestClient_OtherNotFoundIsNotTheShapeTagSentinel(t *testing.T) {
	client, _ := vsServer(t, http.StatusNotFound, `{"notFound":{"type":"item","id":"VX-1"}}`)

	_, err := client.AddShapeToItem("original", "VX-1", "VX-2")

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrShapeTagNotFound))
}

// The carve-out. FileExistsInStorage is a probe whose caller polls on false, so a 404
// has to stay a false rather than becoming an activity failure.
func TestClient_FileExistsInStorageTolerates404(t *testing.T) {
	// GetAbsoluteStoragePath runs first and needs a 200, so vary by path.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("path") != "" {
			// the file search itself
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"notFound":{"type":"file"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"method":[{"uri":"file:///mnt/isilon/"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "pass")

	exists, err := client.FileExistsInStorage("VX-42", "/mnt/isilon/some/file.mxf")

	require.NoError(t, err, "a 404 from the probe must stay a false, not an error")
	assert.False(t, exists)
}

// A 500 on the same probe is still an error: "not visible yet" and "Vidispine is
// broken" must not look the same, or the caller polls for its full 10 minutes and
// then fails with a timeout instead of the real cause.
func TestClient_FileExistsInStorageStillErrorsOn500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("path") != "" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"internalServer":"boom"}`))
			return
		}
		_, _ = w.Write([]byte(`{"method":[{"uri":"file:///mnt/isilon/"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "pass")

	_, err := client.FileExistsInStorage("VX-42", "/mnt/isilon/some/file.mxf")

	require.Error(t, err, "a 500 must be distinguishable from 'not found'")
}

// Deleting something already gone is the outcome we wanted.
func TestClient_DeleteShapeTolerates404(t *testing.T) {
	client, _ := vsServer(t, http.StatusNotFound, `{"notFound":{"type":"shape"}}`)

	err := client.DeleteShape("VX-1", "VX-2")

	assert.NoError(t, err, "an already-deleted shape is not a failure")
}

func TestClient_DeleteShapeStillErrorsOn500(t *testing.T) {
	client, _ := vsServer(t, http.StatusInternalServerError, `{"internalServer":"boom"}`)

	err := client.DeleteShape("VX-1", "VX-2")

	assert.Error(t, err)
}

func TestClient_TimeoutOutlastsASlowVidispineCall(t *testing.T) {
	client, _ := vsServer(t, http.StatusOK, `{}`)

	timeout := client.restyClient.GetClient().Timeout

	assert.GreaterOrEqual(t, timeout, httpx.DefaultTimeout,
		"a POST that is slow rather than broken must finish, not be retried into a duplicate")
}

func TestClient_RetriesAreStillConfigured(t *testing.T) {
	client, _ := vsServer(t, http.StatusOK, `{}`)

	assert.Positive(t, client.restyClient.RetryCount)
}

func TestClient_ErrorStatusIsNotRetried(t *testing.T) {
	client, calls := vsServer(t, http.StatusInternalServerError, `{"internalServer":"boom"}`)

	_, err := client.AddShapeToItem("original", "VX-1", "VX-2")

	require.Error(t, err)
	assert.Equal(t, 1, *calls, "the server answered; asking again would create a second shape")
}

// A successful response must be untouched by the hook.
func TestClient_SuccessIsUnaffected(t *testing.T) {
	client, _ := vsServer(t, http.StatusOK,
		`{"id":"VX-1","terse":{"title":[{"value":"Hello"}]}}`)

	result, err := client.GetMetadata("VX-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Hello", result.Get(vscommon.FieldTitle, "fallback"))
}
