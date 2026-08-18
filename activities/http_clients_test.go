package activities

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bcc-code/bcc-media-flows/environment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func shortsServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	t.Setenv("SHORTS_SERVICE_URL", server.URL)
	environment.Load()
	t.Cleanup(func() { environment.Load() })
}

func activityEnv(t *testing.T) *testsuite.TestActivityEnvironment {
	t.Helper()

	suite := &testsuite.WorkflowTestSuite{}
	return suite.NewTestActivityEnvironment()
}

func TestSubmitShortJobActivity_ServerErrorIsAnError(t *testing.T) {
	shortsServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"boom"}`))
	})

	env := activityEnv(t)
	ua := UtilActivities{}
	env.RegisterActivity(ua.SubmitShortJobActivity)

	_, err := env.ExecuteActivity(ua.SubmitShortJobActivity, SubmitShortJobInput{InputPath: "/mnt/in.mp4"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestSubmitShortJobActivity_AcceptedReturnsTheJobID(t *testing.T) {
	shortsServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/submit_job", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"job_id":"job-1"}`))
	})

	env := activityEnv(t)
	ua := UtilActivities{}
	env.RegisterActivity(ua.SubmitShortJobActivity)

	val, err := env.ExecuteActivity(ua.SubmitShortJobActivity, SubmitShortJobInput{InputPath: "/mnt/in.mp4"})
	require.NoError(t, err)

	var result SubmitShortJobResult
	require.NoError(t, val.Get(&result))
	assert.Equal(t, "job-1", result.JobID)
}

func TestSubmitShortJobActivity_SuccessWithoutAJobIDIsAnError(t *testing.T) {
	shortsServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{}`))
	})

	env := activityEnv(t)
	ua := UtilActivities{}
	env.RegisterActivity(ua.SubmitShortJobActivity)

	_, err := env.ExecuteActivity(ua.SubmitShortJobActivity, SubmitShortJobInput{InputPath: "/mnt/in.mp4"})

	require.Error(t, err, "GenerateShort would otherwise poll job \"\" until it gives up")
	assert.Contains(t, err.Error(), "no job id")
}

func TestCheckJobStatusActivity_SuccessParsesTheKeyframes(t *testing.T) {
	shortsServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/job_status/job-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"done","keyframes":[
			{"start_timestamp":0.5,"end_timestamp":2.5,"x":10,"y":20,"w":100,"h":200}]}`))
	})

	env := activityEnv(t)
	ua := UtilActivities{}
	env.RegisterActivity(ua.CheckJobStatusActivity)

	val, err := env.ExecuteActivity(ua.CheckJobStatusActivity, CheckJobStatusInput{JobID: "job-1"})
	require.NoError(t, err)

	var result GenerateShortRequestResult
	require.NoError(t, val.Get(&result))
	assert.Equal(t, "done", result.Status)
	require.Len(t, result.Keyframes, 1)
	assert.Equal(t, 2.5, result.Keyframes[0].EndTimestamp)
	assert.Equal(t, 100, result.Keyframes[0].W)
}

func TestCheckJobStatusActivity_NotFoundIsAnError(t *testing.T) {
	shortsServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"unknown job"}`))
	})

	env := activityEnv(t)
	ua := UtilActivities{}
	env.RegisterActivity(ua.CheckJobStatusActivity)

	_, err := env.ExecuteActivity(ua.CheckJobStatusActivity, CheckJobStatusInput{JobID: "job-1"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestGetAudioDiff_ConvertsSecondsToMilliseconds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"offset":1.234}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("SYNC_SERVICE_URL", server.URL)

	env := activityEnv(t)
	ua := UtilActivities{}
	env.RegisterActivity(ua.GetAudioDiff)

	val, err := env.ExecuteActivity(ua.GetAudioDiff, GetAudioDiffParams{
		ReferenceFile: "/mnt/ref.wav",
		TargetFile:    "/mnt/target.wav",
	})
	require.NoError(t, err)

	var result GetAudioDiffResult
	require.NoError(t, val.Get(&result))
	assert.Equal(t, 1234, result.Difference)
}

func TestGetAudioDiff_ServerErrorIsNotAZeroOffset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"boom"}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("SYNC_SERVICE_URL", server.URL)

	env := activityEnv(t)
	ua := UtilActivities{}
	env.RegisterActivity(ua.GetAudioDiff)

	_, err := env.ExecuteActivity(ua.GetAudioDiff, GetAudioDiffParams{
		ReferenceFile: "/mnt/ref.wav",
		TargetFile:    "/mnt/target.wav",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestGetAudioDiff_SendsBothFiles(t *testing.T) {
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(raw)
		body = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"offset":0}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("SYNC_SERVICE_URL", server.URL)

	env := activityEnv(t)
	ua := UtilActivities{}
	env.RegisterActivity(ua.GetAudioDiff)

	_, err := env.ExecuteActivity(ua.GetAudioDiff, GetAudioDiffParams{
		ReferenceFile: "/mnt/ref.wav",
		TargetFile:    "/mnt/target.wav",
	})

	require.NoError(t, err)
	assert.Contains(t, body, `"reference_file":"/mnt/ref.wav"`)
	assert.Contains(t, body, `"target_file":"/mnt/target.wav"`)
}

func TestServiceClientsHaveTimeouts(t *testing.T) {
	assert.Positive(t, shortServiceClient().GetClient().Timeout)
}
