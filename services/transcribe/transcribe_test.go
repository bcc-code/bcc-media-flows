package transcribe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
)

func transcribeServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	previous := baseURL
	baseURL = server.URL
	t.Cleanup(func() { baseURL = previous })
}

func fastPolling(t *testing.T) {
	t.Helper()

	previous := pollInterval
	pollInterval = time.Millisecond
	t.Cleanup(func() { pollInterval = previous })
}

func TestDoTranscribe_RejectsMissingArguments(t *testing.T) {
	_, err := DoTranscribe(context.Background(), "", "/mnt/out", "no")
	assert.ErrorIs(t, err, errNoInputFile)

	_, err = DoTranscribe(context.Background(), "/mnt/in.wav", "", "no")
	assert.ErrorIs(t, err, errNoOutput)
}

func TestDoTranscribe_SubmitFailureIsAnError(t *testing.T) {
	transcribeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"boom"}`))
	})

	job, err := DoTranscribe(context.Background(), "/mnt/in.wav", "/mnt/out", "no")

	require.Error(t, err)
	assert.Nil(t, job)
	assert.Contains(t, err.Error(), "500")
}

func TestDoTranscribe_SubmitWithoutAnIDIsAnError(t *testing.T) {
	transcribeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"QUEUED"}`))
	})

	job, err := DoTranscribe(context.Background(), "/mnt/in.wav", "/mnt/out", "no")

	require.Error(t, err)
	assert.Nil(t, job)
	assert.Contains(t, err.Error(), "no id")
}

func TestDoTranscribe_PollNotFoundEndsTheLoop(t *testing.T) {
	fastPolling(t)

	var polls atomic.Int32
	transcribeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"job-1","status":"QUEUED"}`))
			return
		}
		polls.Add(1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"unknown job"}`))
	})

	done := make(chan struct{})
	var err error
	go func() {
		_, err = DoTranscribe(context.Background(), "/mnt/in.wav", "/mnt/out", "no")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("DoTranscribe kept polling a job that returns 404")
	}

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.LessOrEqual(t, polls.Load(), int32(2), "one failed poll is enough to know")
}

func TestDoTranscribe_PollsUntilCompleted(t *testing.T) {
	fastPolling(t)

	var polls atomic.Int32
	transcribeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"job-1","status":"QUEUED"}`))
			return
		}
		if polls.Add(1) < 3 {
			_, _ = w.Write([]byte(`{"id":"job-1","status":"PROCESSING","progress":40}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"job-1","status":"COMPLETED","result":"/mnt/out/in.json"}`))
	})

	job, err := DoTranscribe(context.Background(), "/mnt/in.wav", "/mnt/out", "no")

	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, "COMPLETED", job.Status)
	assert.Equal(t, "/mnt/out/in.json", job.Result)
	assert.Equal(t, int32(3), polls.Load())
}

func TestDoTranscribe_FailedJobIsAnError(t *testing.T) {
	fastPolling(t)

	transcribeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"job-1","status":"QUEUED"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"job-1","status":"FAILED"}`))
	})

	job, err := DoTranscribe(context.Background(), "/mnt/in.wav", "/mnt/out", "no")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "job-1", "the error names the job that failed")
	assert.NotNil(t, job, "the failed job is still returned for its detail")
}

func TestDoTranscribe_CancellationEndsTheLoop(t *testing.T) {
	previous := pollInterval
	pollInterval = time.Hour
	t.Cleanup(func() { pollInterval = previous })

	transcribeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"job-1","status":"QUEUED"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"job-1","status":"PROCESSING"}`))
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := DoTranscribe(ctx, "/mnt/in.wav", "/mnt/out", "no")
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("a cancelled context did not end the poll loop")
	}
}

func TestDoTranscribe_SendsTheJobDefinition(t *testing.T) {
	fastPolling(t)

	var submitted string
	transcribeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			body := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(body)
			submitted = string(body)
			_, _ = w.Write([]byte(`{"id":"job-1"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"job-1","status":"COMPLETED"}`))
	})

	_, err := DoTranscribe(context.Background(), "/mnt/in.wav", "/mnt/out", "nor")

	require.NoError(t, err)
	assert.Contains(t, submitted, `"path":"/mnt/in.wav"`)
	assert.Contains(t, submitted, `"output_path":"/mnt/out"`)
	assert.Contains(t, submitted, `"format":"all"`)
}

func TestClient_RetryWaitsAreSecondsNotNanoseconds(t *testing.T) {
	client := newClient()

	assert.GreaterOrEqual(t, client.RetryWaitTime, time.Second)
	assert.GreaterOrEqual(t, client.RetryMaxWaitTime, client.RetryWaitTime)
	assert.Positive(t, client.RetryCount)
}

func TestClient_DebugIsOff(t *testing.T) {
	client := newClient()

	assert.False(t, client.Debug)
}

func TestClient_HasATimeout(t *testing.T) {
	client := newClient()

	assert.Positive(t, client.GetClient().Timeout)
}

func TestDoTranscribe_HeartbeatsWhileItPolls(t *testing.T) {
	fastPolling(t)

	var polls atomic.Int32
	transcribeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"job-1","status":"QUEUED"}`))
			return
		}
		if polls.Add(1) < 4 {
			_, _ = w.Write([]byte(`{"id":"job-1","status":"PROCESSING"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"job-1","status":"COMPLETED"}`))
	})

	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()

	var heartbeats atomic.Int32
	env.SetOnActivityHeartbeatListener(func(*activity.Info, converter.EncodedValues) {
		heartbeats.Add(1)
	})

	transcribeActivity := func(ctx context.Context) (string, error) {
		job, err := DoTranscribe(ctx, "/mnt/in.wav", "/mnt/out", "no")
		if err != nil {
			return "", err
		}
		return job.Status, nil
	}
	env.RegisterActivity(transcribeActivity)

	val, err := env.ExecuteActivity(transcribeActivity)
	require.NoError(t, err)

	var status string
	require.NoError(t, val.Get(&status))
	assert.Equal(t, "COMPLETED", status)

	assert.NotZero(t, heartbeats.Load(), "a long transcription must heartbeat while it polls")
}

func TestNormalizeTranscriptionLanguage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "supported code is kept", input: "en", want: "en"},
		{name: "case is normalised", input: "DE", want: "de"},
		{name: "norwegian two-letter is supported", input: "NO", want: "no"},
		{name: "three-letter code is not one of whisper's", input: "nor", want: "auto"},
		{name: "unknown falls back to auto", input: "klingon", want: "auto"},
		{name: "auto is passed through", input: "auto", want: "auto"},
		{name: "empty is passed through", input: "", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, normalizeTranscriptionLanguage(test.input))
		})
	}
}
