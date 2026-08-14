package rclone

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rcloneServer points the package at a stub for the duration of one test and returns
// the bodies it received, in order.
func rcloneServer(t *testing.T, handler http.HandlerFunc) *[]string {
	t.Helper()

	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		if r.ContentLength > 0 {
			_, _ = r.Body.Read(body)
		}
		bodies = append(bodies, string(body))
		handler(w, r)
	}))
	t.Cleanup(server.Close)

	previous := baseUrl
	baseUrl = server.URL
	t.Cleanup(func() { baseUrl = previous })

	return &bodies
}

// fastRetries keeps the retry loop from waiting five seconds a turn.
func fastRetries(t *testing.T) {
	t.Helper()

	previous := retryWait
	retryWait = time.Millisecond
	t.Cleanup(func() { retryWait = previous })
}

func TestClient_HasATimeout(t *testing.T) {
	assert.Positive(t, client.Timeout,
		"every file copy in the tree goes through this client")
}

// CopyDir rather than CopyFile: the per-file calls wait for a slot from the transfer
// queue, which only the worker's StartFileTransferQueue goroutine grants.
func TestCopyDir_SubmitsAnAsyncJobAndReturnsItsID(t *testing.T) {
	bodies := rcloneServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sync/copy", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jobid":42}`))
	})

	res, err := CopyDir(context.Background(), "src:/dir", "dst:/dir")

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 42, res.JobID)
	require.Len(t, *bodies, 1)
	assert.Contains(t, (*bodies)[0], `"_async":true`)
	assert.Contains(t, (*bodies)[0], `"srcFs":"src:/dir"`)
}

// A cancelled activity must not sit in the transfer queue, which waits up to an hour
// for a slot before the request is even made.
func TestCopyFile_CancellationEndsTheQueueWait(t *testing.T) {
	rcloneServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jobid":42}`))
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() {
		_, err := CopyFile(ctx, "src:", "a.mxf", "dst:", "b.mxf", PriorityHigh)
		done <- err
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("a cancelled context did not end the wait for a transfer slot")
	}
}

// A failed submit must not read as a job that started: the caller polls the id it gets
// back, and a zero id is a job rclone knows nothing about.
func TestCopyDir_ServerErrorIsAnError(t *testing.T) {
	rcloneServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"directory not found"}`))
	})

	res, err := CopyDir(context.Background(), "src:/dir", "dst:/dir")

	require.Error(t, err)
	assert.Nil(t, res)
}

// The error names the request and quotes the body, because "non-200 status" on its own
// leaves an operator with nothing to act on.
func TestDoRequest_ErrorDescribesTheFailure(t *testing.T) {
	rcloneServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"directory not found"}`))
	})

	_, err := ListFiles(context.Background(), "remote:", "/missing")

	require.Error(t, err)
	assert.ErrorIs(t, err, errNon200Status, "the sentinel survives")
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "/operations/list")
	assert.Contains(t, err.Error(), "directory not found")
}

// A body large enough to be a problem is bounded rather than carried whole into the
// workflow history.
func TestDoRequest_ErrorBodyIsBounded(t *testing.T) {
	rcloneServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(strings.Repeat("x", 50_000)))
	})

	_, err := ListFiles(context.Background(), "remote:", "/x")

	require.Error(t, err)
	assert.Less(t, len(err.Error()), 4096)
}

// Every attempt has to carry the job id. The request body is a reader, so a request
// that is sent once and retried asks about no job at all — and rclone answers that
// with a 500 the caller then reports as a failed copy.
func TestCheckJobStatus_EveryAttemptAsksAboutTheJob(t *testing.T) {
	var attempts atomic.Int32
	bodies := rcloneServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"busy"}`))
			return
		}
		_, _ = w.Write([]byte(`{"finished":true,"success":true,"id":7}`))
	})

	fastRetries(t)

	status, err := CheckJobStatus(context.Background(), 7, 5)

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.Finished)
	require.Len(t, *bodies, 3)
	for i, body := range *bodies {
		assert.Contains(t, body, `"jobid":7`, "attempt %d asked about no job", i+1)
	}
}

func TestCheckJobStatus_GivesUpAfterTheLastAttempt(t *testing.T) {
	var attempts atomic.Int32
	rcloneServer(t, func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	})

	fastRetries(t)

	_, err := CheckJobStatus(context.Background(), 7, 3)

	require.Error(t, err)
	assert.Equal(t, int32(3), attempts.Load())
}

// A cancelled activity must not keep waiting out the retry.
func TestCheckJobStatus_CancellationEndsTheRetries(t *testing.T) {
	rcloneServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CheckJobStatus(ctx, 7, 5)

	require.Error(t, err)
}

func TestStat_ReturnsTheFile(t *testing.T) {
	rcloneServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/operations/stat", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"item":{"Name":"a.mxf","Size":1024,"IsDir":false}}`))
	})

	file, err := Stat(context.Background(), "remote:", "a.mxf")

	require.NoError(t, err)
	require.NotNil(t, file)
	assert.Equal(t, "a.mxf", file.Name)
	assert.Equal(t, 1024, file.Size)
}

func TestListFiles_ReturnsTheListing(t *testing.T) {
	rcloneServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"list":[{"Name":"a.mxf"},{"Name":"b.mxf"}]}`))
	})

	files, err := ListFiles(context.Background(), "remote:", "/dir")

	require.NoError(t, err)
	assert.Len(t, files, 2)
}

// The transfer queue is not an HTTP wait, but it is the longest one in this package —
// an hour — and a cancelled activity should not sit in it.
func TestWaitForTransferSlot_RespectsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitForTransferSlot(ctx, PriorityLow, time.Hour)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
