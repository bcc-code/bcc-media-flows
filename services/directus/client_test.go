package directus

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// directusServer answers every request the same way.
func directusServer(t *testing.T, status int, body string) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return NewClient(server.URL, "test-api-key")
}

// recordingServer captures the request each method makes and answers with body.
func recordingServer(t *testing.T, body string) (*Client, *http.Request) {
	t.Helper()

	captured := &http.Request{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*captured = *r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return NewClient(server.URL, "test-api-key"), captured
}

// The client decides on the status for every method, so a method added later is safe
// without its author writing a check. These cases pin that, one per method.
func TestClient_ServerErrorsAreErrors(t *testing.T) {
	tests := []struct {
		name string
		call func(*Client) error
	}{
		{"GetAssetByMediabankenID", func(c *Client) error { _, err := c.GetAssetByMediabankenID("MB-1"); return err }},
		{"AssetExists", func(c *Client) error { _, err := c.AssetExists("MB-1"); return err }},
		{"CreateStyledImage", func(c *Client) error { _, err := c.CreateStyledImage("file-1", "poster"); return err }},
		{"CreateShort", func(c *Client) error { _, err := c.CreateShort(ShortCreate{MediaItemID: "mi-1"}); return err }},
		{"CreateMediaItemStyledImage", func(c *Client) error { return c.CreateMediaItemStyledImage("mi-1", "si-1") }},
		{"CreateMediaItem", func(c *Client) error { _, err := c.CreateMediaItem(MediaItemCreate{Label: "l"}); return err }},
		{"GetTagByCode", func(c *Client) error { _, err := c.GetTagByCode("code"); return err }},
		{"CreateMediaItemTag", func(c *Client) error { _, err := c.CreateMediaItemTag("mi-1", "tag-1"); return err }},
		{"CreateTag", func(c *Client) error { _, err := c.CreateTag("code", "name"); return err }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := directusServer(t, http.StatusInternalServerError, `{"errors":[{"message":"boom"}]}`)

			err := test.call(client)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "500")
			assert.Contains(t, err.Error(), "directus")
		})
	}
}

// An HTML error page from a proxy is the case a JSON-shaped check would miss.
func TestClient_HTMLErrorBodyIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>502 Bad Gateway</html>"))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "test-api-key")
	asset, err := client.GetAssetByMediabankenID("MB-1")

	require.Error(t, err)
	assert.Nil(t, asset)
}

// 201 Created is what a REST API is entitled to answer a POST with, and it means the
// item exists. Reporting that as a failure invites a retry, which is how duplicates
// get made.
func TestClient_CreatedIsASuccess(t *testing.T) {
	client := directusServer(t, http.StatusCreated, `{"data":{"id":"tag-1","code":"c","name":"n"}}`)

	tag, err := client.CreateTag("c", "n")

	require.NoError(t, err, "201 means the tag was created")
	require.NotNil(t, tag)
	assert.Equal(t, "tag-1", tag.ID)
}

// A genuinely absent asset is not an error: the caller creates one.
func TestGetAssetByMediabankenID_EmptyResultIsNotAnError(t *testing.T) {
	client := directusServer(t, http.StatusOK, `{"data":[]}`)

	asset, err := client.GetAssetByMediabankenID("MB-1")

	require.NoError(t, err)
	assert.Nil(t, asset)
}

func TestGetAssetByMediabankenID_SuccessParsesTheAsset(t *testing.T) {
	client := directusServer(t, http.StatusOK, `{"data":[{"id":7,"mediabanken_id":"MB-1"}]}`)

	asset, err := client.GetAssetByMediabankenID("MB-1")

	require.NoError(t, err)
	require.NotNil(t, asset)
	assert.Equal(t, int64(7), asset.ID)
	assert.Equal(t, "MB-1", asset.MediabankenID)
}

func TestAssetExists(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		client := directusServer(t, http.StatusOK, `{"data":[{"id":7}]}`)

		exists, err := client.AssetExists("MB-1")

		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("absent", func(t *testing.T) {
		client := directusServer(t, http.StatusOK, `{"data":[]}`)

		exists, err := client.AssetExists("MB-1")

		require.NoError(t, err)
		assert.False(t, exists)
	})
}

// The paths are relative to the client's base URL, so one of each verb is checked
// against the full path the service actually receives.
func TestClient_RequestsLandOnTheExpectedPaths(t *testing.T) {
	t.Run("GET carries its filter", func(t *testing.T) {
		client, captured := recordingServer(t, `{"data":[]}`)

		_, err := client.GetTagByCode("my-code")

		require.NoError(t, err)
		assert.Equal(t, "/items/tags", captured.URL.Path)
		assert.Equal(t, "my-code", captured.URL.Query().Get("filter[code][_eq]"))
		assert.Equal(t, http.MethodGet, captured.Method)
	})

	t.Run("POST", func(t *testing.T) {
		client, captured := recordingServer(t, `{"data":{"id":"mi-1"}}`)

		_, err := client.CreateMediaItem(MediaItemCreate{Label: "label"})

		require.NoError(t, err)
		assert.Equal(t, "/items/mediaitems", captured.URL.Path)
		assert.Equal(t, http.MethodPost, captured.Method)
	})
}

func TestClient_SendsTheBearerToken(t *testing.T) {
	client, captured := recordingServer(t, `{"data":[]}`)

	_, err := client.AssetExists("MB-1")

	require.NoError(t, err)
	assert.Equal(t, "Bearer test-api-key", captured.Header.Get("Authorization"))
}

// CreateStyledImage validates its style before making a request.
func TestCreateStyledImage_RejectsAnUnknownStyleWithoutCallingDirectus(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "test-api-key")
	_, err := client.CreateStyledImage("file-1", "banner")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid style")
	assert.Zero(t, calls)
}

// A 200 whose body carries no id is not a created image, and the caller would otherwise
// go on to reference an empty id.
func TestCreateStyledImage_MissingIDIsAnError(t *testing.T) {
	client := directusServer(t, http.StatusOK, `{"data":{}}`)

	_, err := client.CreateStyledImage("file-1", "poster")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing styled image ID")
}

func TestUploadFile_ServerErrorIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "poster.jpg")
	require.NoError(t, os.WriteFile(path, []byte("not really a jpeg"), 0644))

	client := directusServer(t, http.StatusInternalServerError, `{"errors":[]}`)

	file, err := client.UploadFile("folder-1", path)

	require.Error(t, err)
	assert.Nil(t, file)
}

func TestUploadFile_SendsTheFileAndParsesTheResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "poster.jpg")
	require.NoError(t, os.WriteFile(path, []byte("jpeg-bytes"), 0644))

	var uploaded, folder string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, header, err := r.FormFile("file")
		if err == nil {
			defer func() { _ = file.Close() }()
			uploaded = header.Filename
		}
		folder = r.FormValue("folder")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"file-1"}}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "test-api-key")
	file, err := client.UploadFile("folder-1", path)

	require.NoError(t, err)
	require.NotNil(t, file)
	assert.Equal(t, "file-1", file.ID)
	assert.Equal(t, "poster.jpg", uploaded)
	assert.Equal(t, "folder-1", folder)
}

// The upload is the slowest thing this client does, so the timeout has to outlast it
// rather than the small JSON calls around it.
func TestClient_TimeoutCoversTheUpload(t *testing.T) {
	client := NewClient("http://directus.example", "test-api-key")

	assert.GreaterOrEqual(t, client.client.GetClient().Timeout, uploadTimeout)
}
