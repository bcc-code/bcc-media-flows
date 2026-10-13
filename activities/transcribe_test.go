package activities

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/transcribe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
)

type transcriberConfig string

func (c transcriberConfig) Transcriber() string { return string(c) }

// transcriberServer answers the submit with job-1 and every poll with pollStatus,
// and reports the Idempotency-Key the submit carried.
func transcriberServer(t *testing.T, pollStatus string) (UtilActivities, *string) {
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

	return UtilActivities{Transcriber: client}, &key
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
	ua := UtilActivities{}
	env.RegisterActivity(ua.Transcribe)

	_, err := env.ExecuteActivity(ua.Transcribe, transcribeParams())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "transcriber client not configured")

	var appErr *temporal.ApplicationError
	require.True(t, errors.As(err, &appErr))
	assert.True(t, appErr.NonRetryable())
}

func TestTranscribe_CanceledJobIsNotRetried(t *testing.T) {
	ua, _ := transcriberServer(t, "CANCELED")
	env := activityEnv(t)
	env.RegisterActivity(ua.Transcribe)

	_, err := env.ExecuteActivity(ua.Transcribe, transcribeParams())

	require.Error(t, err)
	var appErr *temporal.ApplicationError
	require.True(t, errors.As(err, &appErr))
	assert.True(t, appErr.NonRetryable(), "a retry would submit the job again and undo the cancel")
}

func TestTranscribe_FailedJobIsRetried(t *testing.T) {
	ua, _ := transcriberServer(t, "FAILED")
	env := activityEnv(t)
	env.RegisterActivity(ua.Transcribe)

	_, err := env.ExecuteActivity(ua.Transcribe, transcribeParams())

	require.Error(t, err)
	var appErr *temporal.ApplicationError
	require.True(t, errors.As(err, &appErr))
	assert.False(t, appErr.NonRetryable(), "a failed job may succeed on another attempt")
}

func TestTranscribe_SubmitsWithAKeyForThisAttempt(t *testing.T) {
	ua, key := transcriberServer(t, "COMPLETED")
	env := activityEnv(t)
	env.RegisterActivity(ua.Transcribe)

	_, err := env.ExecuteActivity(ua.Transcribe, transcribeParams())

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
	ua := UtilActivities{Transcriber: client}
	env := activityEnv(t)
	env.RegisterActivity(ua.Transcribe)

	val, err := env.ExecuteActivity(ua.Transcribe, transcribeParams())
	require.NoError(t, err)

	var response TranscribeResponse
	require.NoError(t, val.Get(&response))
	assert.Equal(t, paths.MustParse("/mnt/temp/out/renamed.json"), response.JSONPath, "not the assumed in.wav.json")
	assert.Equal(t, paths.MustParse("/mnt/temp/out/renamed.txt"), response.TXTPath)
}

func TestTranscribe_WithoutResultsAssumesTheOldNames(t *testing.T) {
	ua, _ := transcriberServer(t, "COMPLETED")
	env := activityEnv(t)
	env.RegisterActivity(ua.Transcribe)

	val, err := env.ExecuteActivity(ua.Transcribe, transcribeParams())
	require.NoError(t, err)

	var response TranscribeResponse
	require.NoError(t, val.Get(&response))
	assert.Equal(t, paths.MustParse("/mnt/temp/out/in.wav.json"), response.JSONPath)
	assert.Equal(t, paths.MustParse("/mnt/temp/out/in.wav.words.srt"), response.WordsSRTPath)
}
