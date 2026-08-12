package wfutils

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
)

func TestActivityCounterCountsInFlightActivities(t *testing.T) {
	c := &ActivityCounter{}
	assert.Equal(t, 0, c.Running())

	first := c.Started()
	second := c.Started()
	assert.Equal(t, 2, c.Running())

	first()
	assert.Equal(t, 1, c.Running())

	second()
	assert.Equal(t, 0, c.Running())
}

func TestActivityCounterDoneIsIdempotent(t *testing.T) {
	c := &ActivityCounter{}
	done := c.Started()

	done()
	done()
	done()

	assert.Equal(t, 0, c.Running(), "repeated calls must not push the count negative")
}

func TestActivityCounterUnderConcurrency(t *testing.T) {
	c := &ActivityCounter{}

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			done := c.Started()
			assert.GreaterOrEqual(t, c.Running(), 1)
			done()
		}()
	}
	wg.Wait()

	assert.Equal(t, 0, c.Running())
}

type countingActivityInput struct {
	Fail bool
}

// countDuringActivity reports what the counter said while it was running, which
// is the thing the self-update gate reads.
func countDuringActivity(_ context.Context, in countingActivityInput) (int, error) {
	running := RunningActivities.Running()
	if in.Fail {
		return running, assert.AnError
	}
	return running, nil
}

func newCountingActivityEnv(t *testing.T) *testsuite.TestActivityEnvironment {
	t.Helper()

	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.SetWorkerOptions(worker.Options{
		Interceptors: []interceptor.WorkerInterceptor{&AnalyticsWorkerInterceptor{}},
	})
	env.RegisterActivity(countDuringActivity)
	return env
}

// The old accounting lived in wfutils.Execute, on the worker that scheduled the
// activity rather than the one running it. These assert the count is visible
// from inside the activity itself.
func TestRunningActivitiesCountsFromInsideTheActivity(t *testing.T) {
	require.Equal(t, 0, RunningActivities.Running())

	env := newCountingActivityEnv(t)
	val, err := env.ExecuteActivity(countDuringActivity, countingActivityInput{})
	require.NoError(t, err)

	var running int
	require.NoError(t, val.Get(&running))
	assert.Equal(t, 1, running, "the activity should see itself counted")
	assert.Equal(t, 0, RunningActivities.Running(), "and be uncounted once it returns")
}

func TestRunningActivitiesIsDecrementedWhenTheActivityFails(t *testing.T) {
	require.Equal(t, 0, RunningActivities.Running())

	env := newCountingActivityEnv(t)
	_, err := env.ExecuteActivity(countDuringActivity, countingActivityInput{Fail: true})
	require.Error(t, err)

	assert.Equal(t, 0, RunningActivities.Running(),
		"a failed activity must not leak a count, or the worker never self-updates again")
}
