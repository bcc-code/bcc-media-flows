package activities

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bcc-code/bcc-media-flows/services/vizualizer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
)

// vizualizerStub answers status polls with "processing" until the given number
// of polls have happened, then "completed".
func vizualizerStub(t *testing.T, pollsBeforeDone int) (*vizualizer.Client, *atomic.Int32) {
	t.Helper()

	var polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := polls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if int(n) < pollsBeforeDone {
			_, _ = w.Write([]byte(`{"job_id":"job-1","status":"processing","progress":10}`))
			return
		}
		_, _ = w.Write([]byte(`{"job_id":"job-1","status":"completed","progress":100}`))
	}))
	t.Cleanup(server.Close)

	client, err := vizualizer.NewClient(server.URL)
	require.NoError(t, err)
	return client, &polls
}

// A visualization runs as long as the audio, so it routinely outlives the ten
// minute HeartbeatTimeout in GetDefaultActivityOptions. Without a heartbeat per
// poll the activity is killed mid-render and every retry dies the same way.
func TestWaitForVisualizationHeartbeatsWhileItPolls(t *testing.T) {
	client, polls := vizualizerStub(t, 4)

	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()

	var heartbeats atomic.Int32
	var lastStatus vizualizer.JobStatusResponse
	env.SetOnActivityHeartbeatListener(func(_ *activity.Info, details converter.EncodedValues) {
		heartbeats.Add(1)
		if details.HasValues() {
			_ = details.Get(&lastStatus)
		}
	})

	activities := &VizualizerActivities{Client: client}
	env.RegisterActivity(activities.WaitForVisualization)

	val, err := env.ExecuteActivity(activities.WaitForVisualization, WaitForVisualizationArgs{
		JobID:        "job-1",
		PollInterval: time.Millisecond,
		Timeout:      time.Minute,
	})
	require.NoError(t, err)

	var got vizualizer.JobStatusResponse
	require.NoError(t, val.Get(&got))
	assert.Equal(t, "completed", got.Status)

	assert.Greater(t, polls.Load(), int32(1), "the stub should have been polled while processing")

	// The count is not compared against the number of polls: the SDK throttles
	// heartbeats and only forwards a fraction of them to the listener. What
	// matters is that the activity heartbeats at all while the job runs.
	assert.NotZero(t, heartbeats.Load(), "the activity must heartbeat while polling")
	assert.Equal(t, "job-1", lastStatus.JobID,
		"the job status should travel as the heartbeat detail")
}

func TestWaitForVisualizationRequiresAJobID(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()

	activities := &VizualizerActivities{}
	env.RegisterActivity(activities.WaitForVisualization)

	_, err := env.ExecuteActivity(activities.WaitForVisualization, WaitForVisualizationArgs{})
	require.Error(t, err)
}
