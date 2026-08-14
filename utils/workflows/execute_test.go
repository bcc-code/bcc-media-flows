package wfutils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestActivityOptionsWithDefaultsFillsMissingTimeouts(t *testing.T) {
	got := activityOptionsWithDefaults(workflow.ActivityOptions{})

	defaults := GetDefaultActivityOptions()
	assert.Equal(t, defaults.StartToCloseTimeout, got.StartToCloseTimeout)
	assert.Equal(t, defaults.ScheduleToCloseTimeout, got.ScheduleToCloseTimeout)
}

func TestActivityOptionsWithDefaultsKeepsWhatTheWorkflowSet(t *testing.T) {
	set := workflow.ActivityOptions{
		StartToCloseTimeout:    7 * time.Minute,
		ScheduleToCloseTimeout: 9 * time.Minute,
		HeartbeatTimeout:       3 * time.Minute,
	}

	assert.Equal(t, set, activityOptionsWithDefaults(set))
}

// A heartbeat timeout on an activity that never calls RecordHeartbeat fails
// every run longer than the timeout, so it must not be filled in for a workflow
// that did not ask for one.
func TestActivityOptionsWithDefaultsNeverAddsAHeartbeatTimeout(t *testing.T) {
	require.NotZero(t, GetDefaultActivityOptions().HeartbeatTimeout,
		"otherwise this test passes for the wrong reason")

	got := activityOptionsWithDefaults(workflow.ActivityOptions{})

	assert.Zero(t, got.HeartbeatTimeout)
}

// attemptBudget reports how long this attempt is allowed to run, which is the
// start-to-close timeout the activity was scheduled with.
func attemptBudget(ctx context.Context, _ any) (time.Duration, error) {
	info := activity.GetInfo(ctx)
	return info.Deadline.Sub(info.StartedTime).Round(time.Minute), nil
}

// noOptionsWorkflow is a workflow that never calls WithActivityOptions, which is
// the case the defaults exist for.
func noOptionsWorkflow(ctx workflow.Context) (time.Duration, error) {
	return Execute(ctx, attemptBudget, any(nil)).Result(ctx)
}

func TestExecuteAppliesDefaultsWhenTheWorkflowSetsNoOptions(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(attemptBudget)

	env.ExecuteWorkflow(noOptionsWorkflow)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Without the defaults this is 3h: no start-to-close is set, so the server
	// falls back to the schedule-to-close budget and one attempt can eat all of it.
	var budget time.Duration
	require.NoError(t, env.GetWorkflowResult(&budget))
	assert.Equal(t, GetDefaultActivityOptions().StartToCloseTimeout, budget)
}

// The same workflow, but with options set: Execute must not override them.
func setOptionsWorkflow(ctx workflow.Context) (time.Duration, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout:    30 * time.Minute,
		ScheduleToCloseTimeout: 45 * time.Minute,
	})
	return Execute(ctx, attemptBudget, any(nil)).Result(ctx)
}

func TestExecuteKeepsTheOptionsTheWorkflowSet(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(attemptBudget)

	env.ExecuteWorkflow(setOptionsWorkflow)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var budget time.Duration
	require.NoError(t, env.GetWorkflowResult(&budget))
	assert.Equal(t, 30*time.Minute, budget)
}
