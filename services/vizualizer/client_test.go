package vizualizer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeJSON reads a request body into target.
func decodeJSON(r *http.Request, target any) error {
	defer func() { _ = r.Body.Close() }()
	return json.NewDecoder(r.Body).Decode(target)
}

func vizServer(t *testing.T, status int, contentType, body string) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	return client
}

func TestNewClient_RequiresABaseURL(t *testing.T) {
	client, err := NewClient("")

	require.Error(t, err)
	assert.Nil(t, client)
}

// GetJob is polled by VisualizeAudioActivity every few seconds until the job reports
// done. A failure has to be an error rather than a zero-valued status, which would
// leave the activity polling a job that is not there until its timeout.
func TestGetJob_ServerErrorIsAnError(t *testing.T) {
	client := vizServer(t, http.StatusInternalServerError, "application/json", `{"error":"boom"}`)

	job, err := client.GetJob("job-1")

	require.Error(t, err)
	assert.Nil(t, job)
	assert.Contains(t, err.Error(), "500")
}

func TestGetJob_NotFoundIsAnError(t *testing.T) {
	client := vizServer(t, http.StatusNotFound, "application/json", `{"detail":"unknown job"}`)

	_, err := client.GetJob("job-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestGetJob_RequiresAJobID(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	_, err = client.GetJob("")

	require.Error(t, err)
	assert.Zero(t, calls, "an empty id is not worth a request")
}

func TestGetJob_SuccessParsesTheStatus(t *testing.T) {
	client := vizServer(t, http.StatusOK, "application/json",
		`{"job_id":"job-1","status":"completed","progress":100,"output_file":"/mnt/out.mp4"}`)

	job, err := client.GetJob("job-1")

	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, "completed", job.Status)
	assert.Equal(t, 100, job.Progress)
	assert.Equal(t, "/mnt/out.mp4", job.OutputFile)
}

func TestGetJob_RequestsTheJobPath(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"job_id":"job-1"}`))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	_, err = client.GetJob("job-1")

	require.NoError(t, err)
	assert.Equal(t, "/api/status/job-1", path)
}

// A failed create must not return a response with an empty JobID: the caller would
// then poll for a job that was never started.
func TestCreateVisualization_ServerErrorIsAnError(t *testing.T) {
	client := vizServer(t, http.StatusBadRequest, "application/json", `{"message":"bad audio path"}`)

	created, err := client.CreateVisualization(CreateVisualizationRequest{AudioPath: "/mnt/a.wav"})

	require.Error(t, err)
	assert.Nil(t, created)
}

// 202 Accepted is a plausible answer to "start this job", and it is a success.
func TestCreateVisualization_AcceptedIsASuccess(t *testing.T) {
	client := vizServer(t, http.StatusAccepted, "application/json",
		`{"job_id":"job-1","status":"queued"}`)

	created, err := client.CreateVisualization(CreateVisualizationRequest{AudioPath: "/mnt/a.wav"})

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "job-1", created.JobID)
}

func TestCreateVisualization_SendsTheRequestBody(t *testing.T) {
	var body CreateVisualizationRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = decodeJSON(r, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"job_id":"job-1"}`))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	_, err = client.CreateVisualization(CreateVisualizationRequest{
		AudioPath:  "/mnt/a.wav",
		OutputPath: "/mnt/out.mp4",
		Width:      1920,
		Height:     1080,
		FPS:        25,
	})

	require.NoError(t, err)
	assert.Equal(t, "/mnt/a.wav", body.AudioPath)
	assert.Equal(t, 1920, body.Width)
	assert.Equal(t, 25, body.FPS)
}

func TestListJobs(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client := vizServer(t, http.StatusOK, "application/json",
			`[{"job_id":"a"},{"job_id":"b"}]`)

		jobs, err := client.ListJobs()

		require.NoError(t, err)
		assert.Len(t, jobs, 2)
	})

	t.Run("failure is not an empty list", func(t *testing.T) {
		client := vizServer(t, http.StatusBadGateway, "text/html", "<html>502</html>")

		jobs, err := client.ListJobs()

		require.Error(t, err)
		assert.Nil(t, jobs)
	})
}

func TestHealth(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		client := vizServer(t, http.StatusOK, "application/json", `{"status":"ok"}`)

		assert.NoError(t, client.Health())
	})

	t.Run("unhealthy", func(t *testing.T) {
		client := vizServer(t, http.StatusServiceUnavailable, "text/plain", "down")

		err := client.Health()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "503")
	})
}

// This client had no timeout. VisualizeAudioActivity polls it for as long as a
// visualization takes, and a visualizer that accepts the connection and then stops
// answering would have held one poll open for the whole activity.
func TestClient_HasATimeout(t *testing.T) {
	client, err := NewClient("http://vizualizer.example")
	require.NoError(t, err)

	assert.Positive(t, client.client.GetClient().Timeout)
}
