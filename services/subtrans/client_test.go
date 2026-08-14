package subtrans

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// subtransServer answers every request the same way.
func subtransServer(t *testing.T, status int, contentType, body string) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return NewClient(server.URL, "test-api-key")
}

// routingServer answers the story lookup with storyBody and every export request with
// exportStatus/exportBody, which is the shape GetSubtitles needs: one lookup, then one
// request per language.
func routingServer(t *testing.T, storyBody string, exportStatus int, exportBody string) (*Client, *int) {
	t.Helper()

	exports := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/external/export/") {
			exports++
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(exportStatus)
			_, _ = w.Write([]byte(exportBody))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(storyBody))
	}))
	t.Cleanup(server.Close)

	return NewClient(server.URL, "test-api-key"), &exports
}

const oneNorwegianLanguage = `{"id":42,"name":"AABC-%lang%","languages":[{"isoName":"NOR","approved":true}]}`

// GetSubtitlesActivity writes every value of the returned map straight to a .srt file
// on disk, and that file goes on to be imported into Vidispine and burned into video.
// An error page reaching the map is therefore an error page reaching air.
func TestGetSubtitles_ServerErrorIsNotWrittenAsASubtitle(t *testing.T) {
	client, _ := routingServer(t, oneNorwegianLanguage,
		http.StatusInternalServerError, "<html>500 Internal Server Error</html>")

	subs, err := client.GetSubtitles("42", SubTypeSRT, true)

	require.Error(t, err, "an error page must not become a subtitle file")
	assert.Nil(t, subs)
}

// The same hole with a 404, which is what an unknown story id produces.
func TestGetSubtitles_NotFoundIsAnError(t *testing.T) {
	client, _ := routingServer(t, oneNorwegianLanguage, http.StatusNotFound, "")

	subs, err := client.GetSubtitles("42", SubTypeSRT, true)

	require.Error(t, err)
	assert.Nil(t, subs)
}

// A working fetch is unchanged, including the BOM stripping.
func TestGetSubtitles_SuccessReturnsTheSubtitleText(t *testing.T) {
	client, exports := routingServer(t, oneNorwegianLanguage, http.StatusOK,
		"\ufeff1\n00:00:01,000 --> 00:00:02,000\nHei\n")

	subs, err := client.GetSubtitles("42", SubTypeSRT, true)

	require.NoError(t, err)
	require.Contains(t, subs, "NOR")
	assert.Contains(t, subs["NOR"], "Hei")
	assert.NotContains(t, subs["NOR"], "\ufeff", "the BOM is stripped")
	assert.Equal(t, 1, *exports)
}

// Unapproved languages are skipped, except Norwegian, which the service reports as
// unapproved even when it is not.
func TestGetSubtitles_SkipsUnapprovedExceptNorwegian(t *testing.T) {
	story := `{"id":42,"name":"AABC-%lang%","languages":[
		{"isoName":"NOR","approved":false},
		{"isoName":"ENG","approved":false},
		{"isoName":"DEU","approved":true}]}`

	client, exports := routingServer(t, story, http.StatusOK, "text")

	subs, err := client.GetSubtitles("42", SubTypeSRT, true)

	require.NoError(t, err)
	assert.Equal(t, 2, *exports)
	assert.Contains(t, subs, "NOR")
	assert.Contains(t, subs, "DEU")
	assert.NotContains(t, subs, "ENG")
}

// GetOrCreateSubtransID reads an empty result as "no subtitles exist for this file",
// and acts on it: the ingest either fails as non-retryable or, with NoSubsOK,
// continues without subtitles. A Subtrans outage must not be able to say that.
func TestSearchByName_ServerErrorIsNotAnEmptyResult(t *testing.T) {
	client := subtransServer(t, http.StatusBadGateway, "text/html", "<html>502</html>")

	res, err := client.SearchByName("AABC_2024_S01E01")

	require.Error(t, err, "a failed search must not read as 'no subtitles found'")
	assert.Nil(t, res, "an empty slice here is indistinguishable from a real empty answer")
}

func TestSearchByName_SuccessParsesTheResults(t *testing.T) {
	client := subtransServer(t, http.StatusOK, "application/json",
		`[{"id":42,"name":"AABC-%lang%","program":"AABC"}]`)

	res, err := client.SearchByName("AABC_2024_S01E01")

	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, 42, res[0].ID)
}

func TestSearchByID_ServerErrorIsAnError(t *testing.T) {
	client := subtransServer(t, http.StatusInternalServerError, "text/plain", "boom")

	res, err := client.SearchByID("42")

	require.Error(t, err)
	assert.Nil(t, res, "a zero-valued result would read as a story with no languages")
}

// GetFilePrefix names the files written to disk, so a zero value with no error would
// write every subtitle file with an empty prefix.
func TestGetFilePrefix_ServerErrorIsAnError(t *testing.T) {
	client := subtransServer(t, http.StatusInternalServerError, "text/plain", "boom")

	prefix, err := client.GetFilePrefix("42")

	require.Error(t, err)
	assert.Empty(t, prefix)
}

func TestGetFilePrefix_StripsTheLanguagePlaceholder(t *testing.T) {
	client := subtransServer(t, http.StatusOK, "application/json",
		`{"id":42,"name":"AABC_2024_S01E01_%lang%"}`)

	prefix, err := client.GetFilePrefix("42")

	require.NoError(t, err)
	assert.Equal(t, "AABC_2024_S01E01_", prefix)
}

// The key must reach the service on every request, the exports included.
func TestClient_SendsTheAPIKeyOnEveryRequest(t *testing.T) {
	var keys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.URL.Query().Get("key"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(oneNorwegianLanguage))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "test-api-key")
	_, err := client.GetSubtitles("42", SubTypeSRT, true)

	require.NoError(t, err)
	require.Len(t, keys, 2, "the story lookup and the export")
	for _, key := range keys {
		assert.Equal(t, "test-api-key", key)
	}
}

// The key is in the query string, and the error names the request, so without
// redaction every Subtrans failure would write the key into the workflow history.
func TestClient_ErrorDoesNotLeakTheAPIKey(t *testing.T) {
	client := subtransServer(t, http.StatusInternalServerError, "text/plain", "boom")

	_, err := client.SearchByID("42")

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "test-api-key")
	assert.Contains(t, err.Error(), "subtrans", "it still says which service failed")
}

// approvedOnly is a request parameter rather than something applied afterwards, so it
// has to reach the service as the caller set it.
func TestGetSubtitles_PassesApprovedOnlyThrough(t *testing.T) {
	for _, approvedOnly := range []bool{true, false} {
		t.Run(fmt.Sprintf("%t", approvedOnly), func(t *testing.T) {
			var got string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/external/export/") {
					got = r.URL.Query().Get("onlyApproved")
					_, _ = w.Write([]byte("text"))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(oneNorwegianLanguage))
			}))
			t.Cleanup(server.Close)

			client := NewClient(server.URL, "test-api-key")
			_, err := client.GetSubtitles("42", SubTypeSRT, approvedOnly)

			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("%t", approvedOnly), got)
		})
	}
}
