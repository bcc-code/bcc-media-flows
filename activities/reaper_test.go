package activities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reaperServer points the reaper activities at a stub for the duration of one test.
func reaperServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	previous := reaperBaseUrl
	reaperBaseUrl = server.URL
	t.Cleanup(func() { reaperBaseUrl = previous })
}

func TestStartReaper_ReturnsTheSessionID(t *testing.T) {
	reaperServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/start", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"session_id":"session-1"}`))
	})

	sessionID, err := LiveActivities{}.StartReaper(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, "session-1", sessionID)
}

// A 409 means something is already recording, and the id of that session is the answer
// this activity wants. resty unmarshals a result only on 2xx, so the body is decoded by
// hand — this is the case that pins it.
func TestStartReaper_ConflictStillYieldsTheSession(t *testing.T) {
	reaperServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"session_id":"session-already-running"}`))
	})

	sessionID, err := LiveActivities{}.StartReaper(context.Background(), nil)

	require.NoError(t, err, "a session that is already recording is the answer, not a failure")
	assert.Equal(t, "session-already-running", sessionID)
}

// The workflow reports a failed start over Telegram and carries on, so what this
// activity owes it is an error carrying the reason.
func TestStartReaper_ServerErrorIsAnError(t *testing.T) {
	reaperServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"no audio device"}`))
	})

	sessionID, err := LiveActivities{}.StartReaper(context.Background(), nil)

	require.Error(t, err)
	assert.Empty(t, sessionID)
	assert.Contains(t, err.Error(), "500")
}

// A session id that is not a string — or missing — must be an error rather than a
// panic: a panic in an activity fails the task and retries forever.
func TestStartReaper_UnusableSessionIDIsAnErrorNotAPanic(t *testing.T) {
	tests := map[string]string{
		"missing":       `{"status":"started"}`,
		"not_string":    `{"session_id":42}`,
		"null":          `{"session_id":null}`,
		"empty":         `{"session_id":""}`,
		"not_an_object": `"just a string"`,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			reaperServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			})

			require.NotPanics(t, func() {
				sessionID, err := LiveActivities{}.StartReaper(context.Background(), nil)

				assert.Error(t, err)
				assert.Empty(t, sessionID)
			})
		})
	}
}

func TestStopReaper(t *testing.T) {
	t.Run("returns the recorded files", func(t *testing.T) {
		reaperServer(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/stop", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`["/mnt/a.wav","/mnt/b.wav"]`))
		})

		result, err := LiveActivities{}.StopReaper(context.Background(), nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, []string{"/mnt/a.wav", "/mnt/b.wav"}, result.Files)
	})

	t.Run("a failure is not an empty file list", func(t *testing.T) {
		reaperServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		result, err := LiveActivities{}.StopReaper(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestListReaperFiles(t *testing.T) {
	t.Run("asks for the session and returns its files", func(t *testing.T) {
		var sessionID string
		reaperServer(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/files", r.URL.Path)
			sessionID = r.URL.Query().Get("session_id")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`["/mnt/a.wav"]`))
		})

		result, err := LiveActivities{}.ListReaperFiles(context.Background(),
			&ListReaperFilesParams{SessionID: "session-1"})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, []string{"/mnt/a.wav"}, result.Files)
		assert.Equal(t, "session-1", sessionID)
	})

	// The live ingest imports what this returns, so an outage must not read as a
	// session that recorded nothing.
	t.Run("a failure is not an empty file list", func(t *testing.T) {
		reaperServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("<html>502</html>"))
		})

		result, err := LiveActivities{}.ListReaperFiles(context.Background(),
			&ListReaperFilesParams{SessionID: "session-1"})

		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// A body that is not JSON has to say so, rather than leaving an empty list behind.
func TestListReaperFiles_NonJSONBodyIsAnError(t *testing.T) {
	reaperServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("not json"))
	})

	result, err := LiveActivities{}.ListReaperFiles(context.Background(),
		&ListReaperFilesParams{SessionID: "session-1"})

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestReaperClient_HasATimeout(t *testing.T) {
	assert.Positive(t, reaperClient().GetClient().Timeout)
}
