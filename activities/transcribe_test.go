package activities

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/transcribe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"
)

type transcriberConfig string

func (c transcriberConfig) Transcriber() string { return string(c) }

// transcriberServer answers the submit with job-1 and every poll with pollStatus,
// and reports the Idempotency-Key the submit carried.
func transcriberServer(t *testing.T, pollStatus string) (*TranscriberActivities, *string) {
	t.Helper()

	var key string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			key = r.Header.Get("Idempotency-Key")
			_, _ = w.Write([]byte(`{"id":"job-1","status":"QUEUED"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"job-1","status":"` + pollStatus + `","output_path":"/mnt/temp/out"}`))
	}))
	t.Cleanup(server.Close)

	client, err := transcribe.NewClient(transcriberConfig(server.URL))
	require.NoError(t, err)

	return &TranscriberActivities{Client: client}, &key
}

func transcribeParams() TranscribeParams {
	return TranscribeParams{
		File:            paths.MustParse("/mnt/temp/in.wav"),
		DestinationPath: paths.MustParse("/mnt/temp/out"),
		Language:        "no",
	}
}

func TestTranscribe_WithoutAClientIsNotRetried(t *testing.T) {
	env := activityEnv(t)
	ta := &TranscriberActivities{}
	env.RegisterActivity(ta.Transcribe)

	_, err := env.ExecuteActivity(ta.Transcribe, transcribeParams())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "transcriber client not configured")

	var appErr *temporal.ApplicationError
	require.True(t, errors.As(err, &appErr))
	assert.True(t, appErr.NonRetryable())
}

func TestTranscribe_CanceledJobIsNotRetried(t *testing.T) {
	ta, _ := transcriberServer(t, "CANCELED")
	env := activityEnv(t)
	env.RegisterActivity(ta.Transcribe)

	_, err := env.ExecuteActivity(ta.Transcribe, transcribeParams())

	require.Error(t, err)
	var appErr *temporal.ApplicationError
	require.True(t, errors.As(err, &appErr))
	assert.True(t, appErr.NonRetryable(), "a retry would submit the job again and undo the cancel")
}

func TestTranscribe_FailedJobIsRetried(t *testing.T) {
	ta, _ := transcriberServer(t, "FAILED")
	env := activityEnv(t)
	env.RegisterActivity(ta.Transcribe)

	_, err := env.ExecuteActivity(ta.Transcribe, transcribeParams())

	require.Error(t, err)
	var appErr *temporal.ApplicationError
	require.True(t, errors.As(err, &appErr))
	assert.False(t, appErr.NonRetryable(), "a failed job may succeed on another attempt")
}

func TestTranscribe_SubmitsWithAKeyForThisAttempt(t *testing.T) {
	ta, key := transcriberServer(t, "COMPLETED")
	env := activityEnv(t)
	env.RegisterActivity(ta.Transcribe)

	_, err := env.ExecuteActivity(ta.Transcribe, transcribeParams())

	require.NoError(t, err)
	parts := strings.Split(*key, "/")
	require.Len(t, parts, 3, "run ID, activity ID and attempt: %q", *key)
	assert.NotEmpty(t, parts[0], "run ID")
	assert.NotEmpty(t, parts[1], "activity ID")
	assert.Equal(t, "1", parts[2], "the first attempt")
}

func TestTranscriptPaths(t *testing.T) {
	all := []string{
		"/mnt/temp/out/in.wav.words.srt",
		"/mnt/temp/out/in.wav.vtt",
		"/mnt/temp/out/in.wav.txt",
		"/mnt/temp/out/in.wav.srt",
		"/mnt/temp/out/in.wav.json",
	}

	t.Run("each transcript is picked by its format, in any order", func(t *testing.T) {
		response, err := transcriptPaths(all)

		require.NoError(t, err)
		assert.Equal(t, paths.MustParse("/mnt/temp/out/in.wav.json"), response.JSONPath)
		assert.Equal(t, paths.MustParse("/mnt/temp/out/in.wav.srt"), response.SRTPath, "words.srt is not taken for srt")
		assert.Equal(t, paths.MustParse("/mnt/temp/out/in.wav.words.srt"), response.WordsSRTPath)
		assert.Equal(t, paths.MustParse("/mnt/temp/out/in.wav.txt"), response.TXTPath)
	})

	t.Run("a missing transcript is named", func(t *testing.T) {
		_, err := transcriptPaths([]string{"/mnt/temp/out/in.wav.json", "/mnt/temp/out/in.wav.words.srt"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no srt file")
		assert.Contains(t, err.Error(), "no txt file")
		assert.NotContains(t, err.Error(), "no json file")
	})

	t.Run("words.srt is optional", func(t *testing.T) {
		withoutWords := all[1:]

		response, err := transcriptPaths(withoutWords)

		require.NoError(t, err)
		assert.Equal(t, paths.MustParse("/mnt/temp/out/in.wav.words.srt"), response.WordsSRTPath,
			"an empty Path would not decode from the activity result")
		assert.Equal(t, paths.MustParse("/mnt/temp/out/in.wav.srt"), response.SRTPath)
	})

	t.Run("a path outside the known drives is an error", func(t *testing.T) {
		outside := append([]string{"/somewhere/else/in.wav.json"}, all[:4]...)

		_, err := transcriptPaths(outside)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "json file")
	})
}

func TestTranscribe_UsesTheFilesTheTranscriberListed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"job-1","status":"QUEUED"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"job-1","status":"COMPLETED","output_path":"/mnt/temp/out","results":[
			"/mnt/temp/out/renamed.json","/mnt/temp/out/renamed.srt","/mnt/temp/out/renamed.vtt",
			"/mnt/temp/out/renamed.txt","/mnt/temp/out/renamed.words.srt"]}`))
	}))
	t.Cleanup(server.Close)

	client, err := transcribe.NewClient(transcriberConfig(server.URL))
	require.NoError(t, err)
	ta := &TranscriberActivities{Client: client}
	env := activityEnv(t)
	env.RegisterActivity(ta.Transcribe)

	val, err := env.ExecuteActivity(ta.Transcribe, transcribeParams())
	require.NoError(t, err)

	var response TranscribeResponse
	require.NoError(t, val.Get(&response))
	assert.Equal(t, paths.MustParse("/mnt/temp/out/renamed.json"), response.JSONPath, "not the assumed in.wav.json")
	assert.Equal(t, paths.MustParse("/mnt/temp/out/renamed.txt"), response.TXTPath)
}

func TestTranscribe_WithoutResultsAssumesTheOldNames(t *testing.T) {
	ta, _ := transcriberServer(t, "COMPLETED")
	env := activityEnv(t)
	env.RegisterActivity(ta.Transcribe)

	val, err := env.ExecuteActivity(ta.Transcribe, transcribeParams())
	require.NoError(t, err)

	var response TranscribeResponse
	require.NoError(t, val.Get(&response))
	assert.Equal(t, paths.MustParse("/mnt/temp/out/in.wav.json"), response.JSONPath)
	assert.Equal(t, paths.MustParse("/mnt/temp/out/in.wav.words.srt"), response.WordsSRTPath)
}

func TestTranscribe_PassesTheModelOn(t *testing.T) {
	var submitted string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			submitted = string(body)
			_, _ = w.Write([]byte(`{"id":"job-1","status":"QUEUED"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"job-1","status":"COMPLETED","output_path":"/mnt/temp/out"}`))
	}))
	t.Cleanup(server.Close)

	client, err := transcribe.NewClient(transcriberConfig(server.URL))
	require.NoError(t, err)
	ta := &TranscriberActivities{Client: client}
	env := activityEnv(t)
	env.RegisterActivity(ta.Transcribe)

	params := transcribeParams()
	params.Model = "nb-whisper-large"
	_, err = env.ExecuteActivity(ta.Transcribe, params)

	require.NoError(t, err)
	assert.Contains(t, submitted, `"model":"nb-whisper-large"`)
}

// transcriberPolling submits job-1 and answers every poll with pollStatus/pollBody
func transcriberPolling(t *testing.T, pollStatus int, pollBody string) *TranscriberActivities {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"job-1","status":"QUEUED"}`))
			return
		}
		w.WriteHeader(pollStatus)
		_, _ = w.Write([]byte(pollBody))
	}))
	t.Cleanup(server.Close)

	client, err := transcribe.NewClient(transcriberConfig(server.URL))
	require.NoError(t, err)

	return &TranscriberActivities{Client: client}
}

func TestTranscribe_RetriesOnlyWhatAnotherAttemptCouldFix(t *testing.T) {
	tests := []struct {
		name          string
		pollStatus    int
		pollBody      string
		wantRetryable bool
		wantType      string
	}{
		{
			name:       "the transcriber's job timeout is not retried",
			pollStatus: http.StatusOK, pollBody: `{"id":"job-1","status":"FAILED","error":"timeout"}`,
			wantRetryable: false, wantType: TranscriberJobTimeoutErrorType,
		},
		{
			name:       "another failure is retried",
			pollStatus: http.StatusOK, pollBody: `{"id":"job-1","status":"FAILED","error":"write srt: no space left on device"}`,
			wantRetryable: true,
		},
		{
			name:       "a failure that only mentions a timeout is retried",
			pollStatus: http.StatusOK, pollBody: `{"id":"job-1","status":"FAILED","error":"ffprobe: timeout reading header"}`,
			wantRetryable: true,
		},
		{
			name:       "an HTTP failure is retried",
			pollStatus: http.StatusServiceUnavailable, pollBody: `{"error":"timeout"}`,
			wantRetryable: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ta := transcriberPolling(t, test.pollStatus, test.pollBody)
			env := activityEnv(t)
			env.RegisterActivity(ta.Transcribe)

			_, err := env.ExecuteActivity(ta.Transcribe, transcribeParams())

			require.Error(t, err)
			var appErr *temporal.ApplicationError
			require.True(t, errors.As(err, &appErr))
			assert.Equal(t, !test.wantRetryable, appErr.NonRetryable())
			if test.wantType != "" {
				assert.Equal(t, test.wantType, appErr.Type())
				assert.Contains(t, appErr.Error(), "job-1", "the job is named for diagnosis")
			}
		})
	}
}

// recoveryServer is a transcriber holding job-7, a previous attempt's job, which answers
// its GETs with previousReplies in turn (the last one repeats). A POST creates job-new,
// complete on its first poll.
type recoveryServer struct {
	mu       sync.Mutex
	posts    int
	gets     map[string]int
	replies  []recoveryReply
	identity string
}

type recoveryReply struct {
	status int
	body   string
}

func newRecoveryServer(t *testing.T, previousReplies ...recoveryReply) (*recoveryServer, *TranscriberActivities) {
	t.Helper()

	rs := &recoveryServer{gets: map[string]int{}, replies: previousReplies}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.mu.Lock()
		defer rs.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost {
			rs.posts++
			_, _ = w.Write([]byte(`{"id":"job-new","status":"QUEUED"}`))
			return
		}

		id := path.Base(r.URL.Path)
		rs.gets[id]++
		if id == "job-new" {
			_, _ = w.Write([]byte(`{"id":"job-new","status":"COMPLETED","output_path":"/mnt/temp/out","results":["/mnt/temp/out/new.json","/mnt/temp/out/new.srt","/mnt/temp/out/new.txt"]}`))
			return
		}

		reply := rs.replies[min(rs.gets[id], len(rs.replies))-1]
		w.WriteHeader(reply.status)
		_, _ = w.Write([]byte(reply.body))
	}))
	t.Cleanup(server.Close)

	client, err := transcribe.NewClient(transcriberConfig(server.URL))
	require.NoError(t, err)
	rs.identity = client.Identity()

	return rs, &TranscriberActivities{Client: client}
}

func previousJob(status string, extra string) recoveryReply {
	return recoveryReply{status: http.StatusOK, body: `{"id":"job-7","status":"` + status + `"` + extra + `}`}
}

var previousCompleted = previousJob("COMPLETED", `,"output_path":"/mnt/temp/out","results":["/mnt/temp/out/prev.json","/mnt/temp/out/prev.srt","/mnt/temp/out/prev.txt"]`)

func TestTranscribe_ResumesThePreviousAttemptsJob(t *testing.T) {
	tests := []struct {
		name            string
		replies         []recoveryReply
		foreign         bool
		wantPosts       int
		wantJSON        string // empty: the activity fails
		wantRetryable   bool
		wantFailType    string
		wantHeartbeatID string // empty: no heartbeat at all
	}{
		{
			name:      "a running job is polled on",
			replies:   []recoveryReply{previousJob("RUNNING", ""), previousCompleted},
			wantPosts: 0, wantJSON: "/mnt/temp/out/prev.json", wantHeartbeatID: "job-7",
		},
		{
			name:      "a queued job is polled on",
			replies:   []recoveryReply{previousJob("QUEUED", ""), previousCompleted},
			wantPosts: 0, wantJSON: "/mnt/temp/out/prev.json", wantHeartbeatID: "job-7",
		},
		{
			name:      "a completed job returns its result",
			replies:   []recoveryReply{previousCompleted},
			wantPosts: 0, wantJSON: "/mnt/temp/out/prev.json", wantHeartbeatID: "job-7",
		},
		{
			name:      "a job stopped at the transcriber's timeout fails without a retry",
			replies:   []recoveryReply{previousJob("FAILED", `,"error":"timeout"`)},
			wantPosts: 0, wantFailType: TranscriberJobTimeoutErrorType, wantHeartbeatID: "job-7",
		},
		{
			name:      "a job that failed otherwise is replaced",
			replies:   []recoveryReply{previousJob("FAILED", `,"error":"write srt: no space left on device"`)},
			wantPosts: 1, wantJSON: "/mnt/temp/out/new.json", wantHeartbeatID: "job-new",
		},
		{
			name:      "a canceled job is replaced",
			replies:   []recoveryReply{previousJob("CANCELED", "")},
			wantPosts: 1, wantJSON: "/mnt/temp/out/new.json", wantHeartbeatID: "job-new",
		},
		{
			name:      "a job the transcriber no longer knows is replaced",
			replies:   []recoveryReply{{status: http.StatusNotFound, body: `{"error":"not found"}`}},
			wantPosts: 1, wantJSON: "/mnt/temp/out/new.json", wantHeartbeatID: "job-new",
		},
		{
			name:      "an unavailable transcriber submits nothing and is retried",
			replies:   []recoveryReply{{status: http.StatusServiceUnavailable, body: `{"error":"down"}`}},
			wantPosts: 0, wantRetryable: true,
		},
		{
			name:      "a job on another transcriber is left alone and replaced",
			foreign:   true,
			wantPosts: 1, wantJSON: "/mnt/temp/out/new.json", wantHeartbeatID: "job-new",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rs, ta := newRecoveryServer(t, test.replies...)
			previous := &TranscribeHeartbeat{Version: 1, JobID: "job-7", Transcriber: rs.identity}
			if test.foreign {
				previous.Transcriber = "http://10.12.128.99:8888"
			}

			env := activityEnv(t)
			var heartbeats []TranscribeHeartbeat
			env.SetOnActivityHeartbeatListener(func(_ *activity.Info, details converter.EncodedValues) {
				var heartbeat TranscribeHeartbeat
				require.NoError(t, details.Get(&heartbeat), "every heartbeat carries the job")
				heartbeats = append(heartbeats, heartbeat)
			})
			resume := func(ctx context.Context, input TranscribeParams) (*TranscribeResponse, error) {
				return ta.transcribe(ctx, input, previous)
			}
			env.RegisterActivity(resume)

			val, err := env.ExecuteActivity(resume, transcribeParams())

			rs.mu.Lock()
			defer rs.mu.Unlock()
			assert.Equal(t, test.wantPosts, rs.posts, "jobs submitted")
			if test.foreign {
				assert.Zero(t, rs.gets["job-7"], "another transcriber's job ID is never asked about here")
			}

			if test.wantHeartbeatID == "" {
				assert.Empty(t, heartbeats, "nothing heartbeated, so the details survive for the next attempt")
			} else {
				require.NotEmpty(t, heartbeats)
				for _, heartbeat := range heartbeats {
					assert.Equal(t, TranscribeHeartbeat{Version: 1, JobID: test.wantHeartbeatID, Transcriber: rs.identity}, heartbeat)
				}
			}

			if test.wantJSON != "" {
				require.NoError(t, err)
				var response TranscribeResponse
				require.NoError(t, val.Get(&response))
				assert.Equal(t, paths.MustParse(test.wantJSON), response.JSONPath)
				return
			}

			require.Error(t, err)
			var appErr *temporal.ApplicationError
			require.True(t, errors.As(err, &appErr))
			assert.Equal(t, !test.wantRetryable, appErr.NonRetryable())
			if test.wantFailType != "" {
				assert.Equal(t, test.wantFailType, appErr.Type())
			}
		})
	}
}

func TestDecodeTranscribeHeartbeat(t *testing.T) {
	dc := converter.GetDefaultDataConverter()
	from := func(values ...any) func(...any) error {
		payloads, err := dc.ToPayloads(values...)
		require.NoError(t, err)
		return func(details ...any) error { return dc.FromPayloads(payloads, details...) }
	}

	t.Run("the legacy string names no job", func(t *testing.T) {
		heartbeat, err := decodeTranscribeHeartbeat(from("Transcribe"))
		assert.Error(t, err)
		assert.Nil(t, heartbeat)
	})

	t.Run("empty details name no job", func(t *testing.T) {
		heartbeat, _ := decodeTranscribeHeartbeat(from())
		assert.Nil(t, heartbeat)
	})

	t.Run("details without a job or transcriber name no job", func(t *testing.T) {
		heartbeat, err := decodeTranscribeHeartbeat(from(TranscribeHeartbeat{Version: 1, Transcriber: "http://gpu1:8888"}))
		assert.NoError(t, err)
		assert.Nil(t, heartbeat)
	})

	t.Run("typed details name their job", func(t *testing.T) {
		want := TranscribeHeartbeat{Version: 1, JobID: "job-7", Transcriber: "http://gpu1:8888"}

		heartbeat, err := decodeTranscribeHeartbeat(from(want))

		require.NoError(t, err)
		assert.Equal(t, &want, heartbeat)
	})

	t.Run("a later version with more fields still names its job", func(t *testing.T) {
		later := map[string]any{"version": 2, "job_id": "job-7", "transcriber": "http://gpu1:8888", "host": "gpu1"}

		heartbeat, err := decodeTranscribeHeartbeat(from(later))

		require.NoError(t, err)
		require.NotNil(t, heartbeat)
		assert.Equal(t, "job-7", heartbeat.JobID)
	})
}
