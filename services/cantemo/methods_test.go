package cantemo

import (
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func routedServer(t *testing.T, bodyByPrefix map[string]string) (*Client, *[]string) {
	t.Helper()

	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		for prefix, body := range bodyByPrefix {
			if strings.HasPrefix(r.URL.Path, prefix) {
				_, _ = w.Write([]byte(body))
				return
			}
		}

		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"no route in test server"}`))
	}))
	t.Cleanup(server.Close)

	return NewClient(server.URL, "token"), &requested
}

func TestGetFormats_SuccessParsesTheFormats(t *testing.T) {
	client := cantemoServer(t, http.StatusOK, "application/json",
		`{"formats":[{"name":"transcription_json","download_uri":"/dl/1"}]}`)

	formats, err := client.GetFormats("VX-1")

	require.NoError(t, err)
	require.Len(t, formats, 1)
	assert.Equal(t, "transcription_json", formats[0].Name)
}

func TestGetFormats_ServerErrorIsAnError(t *testing.T) {
	client := cantemoServer(t, http.StatusInternalServerError, "text/html", "<html>boom</html>")

	formats, err := client.GetFormats("VX-1")

	require.Error(t, err)
	assert.Nil(t, formats)
}

func TestGetTranscriptionJSON_FollowsTheDownloadURI(t *testing.T) {
	client, requested := routedServer(t, map[string]string{
		"/API/v2/items/VX-1/formats/": `{"formats":[{"name":"transcription_json","download_uri":"/dl/transcript"}]}`,
		"/dl/transcript":              `{"text":"hello there","language":"no"}`,
	})

	transcription, err := client.GetTranscriptionJSON("VX-1")

	require.NoError(t, err)
	require.NotNil(t, transcription)
	assert.Equal(t, "hello there", transcription.Text)
	assert.Contains(t, *requested, "GET /dl/transcript")
}

func TestGetTranscriptionJSON_NoTranscriptionFormatIsEmptyNotAnError(t *testing.T) {
	client, _ := routedServer(t, map[string]string{
		"/API/v2/items/VX-1/formats/": `{"formats":[{"name":"lowres","download_uri":"/dl/lowres"}]}`,
	})

	transcription, err := client.GetTranscriptionJSON("VX-1")

	require.NoError(t, err)
	require.NotNil(t, transcription)
	assert.Empty(t, transcription.Text)
}

func TestGetTranscriptionJSON_DownloadFailureIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/dl/") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"detail":"boom"}`))
			return
		}
		_, _ = w.Write([]byte(`{"formats":[{"name":"transcription_json","download_uri":"/dl/transcript"}]}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "token")
	transcription, err := client.GetTranscriptionJSON("VX-1")

	require.Error(t, err)
	assert.Nil(t, transcription)
}

func TestGetPreviewUrl_ReturnsAnAbsoluteURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"previews":{"shapes":[{"uri":"/preview/VX-1.mp4"}]}}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "token")
	url, err := client.GetPreviewUrl("VX-1")

	require.NoError(t, err)
	assert.Equal(t, server.URL+"/preview/VX-1.mp4", url)
}

func TestGetPreviewUrl_NoPreviewIsEmptyNotAnError(t *testing.T) {
	client := cantemoServer(t, http.StatusOK, "application/json", `{"previews":{"shapes":[]}}`)

	url, err := client.GetPreviewUrl("VX-1")

	require.NoError(t, err)
	assert.Empty(t, url)
}

func TestGetPreviewUrl_ServerErrorIsAnError(t *testing.T) {
	client := cantemoServer(t, http.StatusBadGateway, "text/html", "<html>502</html>")

	url, err := client.GetPreviewUrl("VX-1")

	require.Error(t, err)
	assert.Empty(t, url)
}

func TestGetFieldTags(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client := cantemoServer(t, http.StatusOK, "application/json", `{"tags":["a","b"]}`)

		tags, err := client.GetFieldTags("portal_mf123")

		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, tags)
	})

	t.Run("failure is not an empty tag list", func(t *testing.T) {
		client := cantemoServer(t, http.StatusInternalServerError, "text/plain", "boom")

		tags, err := client.GetFieldTags("portal_mf123")

		require.Error(t, err)
		assert.Nil(t, tags)
	})
}

func TestGetLookupChoices_MapsKeysToValues(t *testing.T) {
	client := cantemoServer(t, http.StatusOK, "application/json",
		`{"choices":[{"key":"k1","value":"Human One"},{"key":"k2","value":"Human Two"}],
		  "more_choices_exist":false}`)

	choices, err := client.GetLookupChoices("group", "field")

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"k1": "Human One", "k2": "Human Two"}, choices)
}

func TestGetLookupChoices_MoreChoicesExistIsAnError(t *testing.T) {
	client := cantemoServer(t, http.StatusOK, "application/json",
		`{"choices":[{"key":"k1","value":"Human One"}],"more_choices_exist":true}`)

	choices, err := client.GetLookupChoices("group", "field")

	require.Error(t, err)
	assert.Len(t, choices, 1, "what did come back is still returned alongside the error")
}

func TestGetFiles_ParsesTheTimestamps(t *testing.T) {
	client := cantemoServer(t, http.StatusOK, "application/json",
		`{"objects":[{"name":"a.mxf","timestamp":"2021-04-20T16:44:51.790+0000"}]}`)

	result, err := client.GetFiles("/mnt/x", "imported", "storage", 0, "")

	require.NoError(t, err)
	require.Len(t, result.Objects, 1)
	assert.Equal(t, 2021, result.Objects[0].Timestamp.Year())
	assert.Equal(t, 44, result.Objects[0].Timestamp.Minute())
}

func TestGetFiles_UnparseableTimestampIsAnError(t *testing.T) {
	client := cantemoServer(t, http.StatusOK, "application/json",
		`{"objects":[{"name":"a.mxf","timestamp":"yesterday"}]}`)

	result, err := client.GetFiles("/mnt/x", "imported", "storage", 0, "")

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestGetFiles_SendsTheListingParameters(t *testing.T) {
	var query neturl.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"objects":[]}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "token")
	_, err := client.GetFiles("/mnt/x", "imported", "isilon", 3, "name:a.mxf")

	require.NoError(t, err)
	assert.Equal(t, "file", query.Get("item_type"))
	assert.Equal(t, "imported", query.Get("import_state"))
	assert.Equal(t, "isilon", query.Get("storage"))
	assert.Equal(t, "3", query.Get("page"))
	assert.Equal(t, "name:a.mxf", query.Get("query"))
}

func TestMoveFileAndRenameFile(t *testing.T) {
	tests := []struct {
		name    string
		call    func(*Client) (string, error)
		wantURL string
	}{
		{
			name:    "MoveFile",
			call:    func(c *Client) (string, error) { return c.MoveFile("VX-1", "SH-1", "src", "dst", "a.mxf") },
			wantURL: "/API/v2/items/VX-1/shape/SH-1/src/move/",
		},
		{
			name:    "RenameFile",
			call:    func(c *Client) (string, error) { return c.RenameFile("VX-1", "SH-1", "src", "dst", "a.mxf") },
			wantURL: "/API/v2/items/VX-1/shape/SH-1/src/rename/",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var path, method, destination string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path, method = r.URL.Path, r.Method
				destination = r.FormValue("destination_storage")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"message":"ok","task_id":"task-1"}`))
			}))
			t.Cleanup(server.Close)

			client := NewClient(server.URL, "token")
			taskID, err := test.call(client)

			require.NoError(t, err)
			assert.Equal(t, "task-1", taskID)
			assert.Equal(t, test.wantURL, path)
			assert.Equal(t, http.MethodPut, method)
			assert.Equal(t, "dst", destination)
		})
	}
}

func TestMoveFile_ServerErrorIsAnError(t *testing.T) {
	client := cantemoServer(t, http.StatusInternalServerError, "text/plain", "boom")

	taskID, err := client.MoveFile("VX-1", "SH-1", "src", "dst", "a.mxf")

	require.Error(t, err)
	assert.Empty(t, taskID)
}

func TestGetTask(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client := cantemoServer(t, http.StatusOK, "application/json",
			`{"task_id":"task-1","state":"FINISHED"}`)

		task, err := client.GetTask("task-1")

		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "FINISHED", task.State)
	})

	t.Run("failure", func(t *testing.T) {
		client := cantemoServer(t, http.StatusInternalServerError, "text/plain", "boom")

		task, err := client.GetTask("task-1")

		require.Error(t, err)
		assert.Nil(t, task)
	})
}

func TestGetACL(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client := cantemoServer(t, http.StatusOK, "application/json",
			`{"acls":[{"id":"acl-1","permission":"READ","source_name":"editors"}],"total":1}`)

		acl, err := client.GetACL("VX-1")

		require.NoError(t, err)
		require.NotNil(t, acl)
		require.Len(t, acl.ACLs, 1)
		assert.Equal(t, "READ", acl.ACLs[0].Permission)
		assert.Equal(t, 1, acl.Total)
	})

	t.Run("failure", func(t *testing.T) {
		client := cantemoServer(t, http.StatusForbidden, "application/json",
			`{"detail":"You do not have permission"}`)

		acl, err := client.GetACL("VX-1")

		require.Error(t, err)
		assert.Nil(t, acl)
	})
}

func TestAddRelation_PostsToTheRelationPath(t *testing.T) {
	var path, query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "token")
	err := client.AddRelation("VX-parent", "VX-child")

	require.NoError(t, err)
	assert.Equal(t, "/API/v2/items/VX-parent/relation/VX-child", path)
	assert.Contains(t, query, "type=portal_metadata_cascade")
}
