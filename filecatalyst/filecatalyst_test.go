package filecatalyst

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFileCatalystTask_ParsesTheConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rs/tasks/task-1", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"taskId":"task-1","congestionControlAggression":6}`))
	}))
	t.Cleanup(server.Close)

	config, err := GetFileCatalystTask(context.Background(), server.URL, "task-1", "user", "pass")

	require.NoError(t, err)
	assert.Equal(t, "task-1", config.TaskID)
	assert.Equal(t, 6, config.CongestionControlAggression)
}

// FileCatalyst authenticates with its own header rather than HTTP basic auth.
func TestClient_SendsTheRESTAuthorizationHeader(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("RESTAuthorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	_, err := GetFileCatalystTask(context.Background(), server.URL, "task-1", "user", "pass")

	require.NoError(t, err)
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("user:pass")), got)
}

// A failed read must not hand back a zero-valued config: UpdateCongestionControlAggression
// writes what it read straight back, so an empty one would blank the whole task.
func TestGetFileCatalystTask_ServerErrorIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"no such task"}`))
	}))
	t.Cleanup(server.Close)

	config, err := GetFileCatalystTask(context.Background(), server.URL, "task-1", "user", "pass")

	require.Error(t, err)
	assert.Empty(t, config.TaskID)
	assert.Contains(t, err.Error(), "500")
}

func TestUpdateFileCatalystTask_PostsTheConfig(t *testing.T) {
	var method, contentType string
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, contentType = r.Method, r.Header.Get("Content-Type")
		body = make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	err := UpdateFileCatalystTask(context.Background(), server.URL, "task-1", "user", "pass",
		FileCatalystTaskConfig{TaskID: "task-1", CongestionControlAggression: 7})

	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, method)
	assert.Equal(t, "application/json", contentType)
	assert.Contains(t, string(body), `"congestionControlAggression":7`)
}

func TestUpdateFileCatalystTask_ServerErrorIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	err := UpdateFileCatalystTask(context.Background(), server.URL, "task-1", "user", "pass",
		FileCatalystTaskConfig{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

// The aggression change is a read-modify-write, and the write has to carry the value
// the caller asked for on top of the configuration that was read.
func TestUpdateCongestionControlAggression_WritesBackTheReadConfig(t *testing.T) {
	var written string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"taskId":"task-1","taskName":"MB_Grow","congestionControlAggression":5}`))
			return
		}
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		written = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	err := UpdateCongestionControlAggression(context.Background(), server.URL, "task-1", "user", "pass", 7)

	require.NoError(t, err)
	assert.Contains(t, written, `"congestionControlAggression":7`)
	assert.Contains(t, written, `"taskName":"MB_Grow"`, "the rest of the task survives the write")
}

// A read that fails must not be followed by a write: that would post an empty
// configuration over a working task.
func TestUpdateCongestionControlAggression_DoesNotWriteAfterAFailedRead(t *testing.T) {
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			writes++
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	err := UpdateCongestionControlAggression(context.Background(), server.URL, "task-1", "user", "pass", 7)

	require.Error(t, err)
	assert.Zero(t, writes)
}

func TestPokeFileCatalyst_RequiresItsEnvironment(t *testing.T) {
	t.Setenv("FILECATALYST_URL", "")
	t.Setenv("FILECATALYST_TASK_ID", "")
	t.Setenv("FILECATALYST_USERNAME", "")
	t.Setenv("FILECATALYST_PASSWORD", "")

	err := PokeFileCatalyst(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required environment variables")
}

// The poke exists to jog a stalled transfer by changing the aggression to something
// else, so the value it writes has to differ from the one it read.
func TestPokeFileCatalyst_WritesADifferentAggression(t *testing.T) {
	var written string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"taskId":"task-1","congestionControlAggression":6}`))
			return
		}
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		written = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	t.Setenv("FILECATALYST_URL", server.URL)
	t.Setenv("FILECATALYST_TASK_ID", "task-1")
	t.Setenv("FILECATALYST_USERNAME", "user")
	t.Setenv("FILECATALYST_PASSWORD", "pass")

	require.NoError(t, PokeFileCatalyst(context.Background()))

	assert.NotContains(t, written, `"congestionControlAggression":6`)
	assert.Regexp(t, `"congestionControlAggression":[57]`, written)
}

func TestClient_HasATimeout(t *testing.T) {
	assert.Positive(t, newClient("http://filecatalyst.example", "user", "pass").GetClient().Timeout)
}
