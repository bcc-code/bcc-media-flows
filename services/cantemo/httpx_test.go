package cantemo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ansel1/merry/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Cantemo that accepts the connection and then stops answering has to fail the
// request rather than hold the activity until its schedule-to-close budget expires —
// three hours, for a workflow that sets no options.
func TestClient_HasATimeout(t *testing.T) {
	client := NewClient("http://cantemo.example", "token")

	assert.Positive(t, client.restyClient.GetClient().Timeout)
}

// The credential is a header here rather than a query parameter, and every call path
// in this package is relative to the base URL.
func TestClient_SendsTheAuthTokenAndAcceptHeaders(t *testing.T) {
	var authToken, accept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authToken = r.Header.Get("Auth-Token")
		accept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"objects":[]}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "secret-token")
	_, err := client.GetFiles("/mnt/x", "imported", "storage", 0, "")

	require.NoError(t, err)
	assert.Equal(t, "secret-token", authToken)
	assert.Equal(t, "application/json", accept)
}

// A trailing slash on CANTEMO_URL must not produce "//API/v2/...": both this client
// and the shared constructor trim it.
func TestClient_BaseURLTrailingSlashIsTrimmed(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL+"/", "token")
	err := client.AddRelation("VX-1", "VX-2")

	require.NoError(t, err)
	assert.Equal(t, "/API/v2/items/VX-1/relation/VX-2", path)
}

// The status decides, not whether resty managed to unmarshal an error envelope —
// checked here through the Cantemo-specific DescribeError in front of the shared
// hook.
func TestClient_DescribeErrorFallsBackToTheSharedDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"unexpected":"envelope"}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "token")
	_, err := client.GetFiles("/mnt/x", "imported", "storage", 0, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cantemo")
	assert.Contains(t, err.Error(), "502")
	assert.Equal(t, http.StatusBadGateway, merry.HTTPCode(err),
		"the status travels with the error so a caller can branch on it")
}
