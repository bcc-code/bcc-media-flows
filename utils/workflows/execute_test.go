package wfutils

import (
	"context"
	"testing"
	"time"

	"github.com/bcc-code/bcc-media-flows/environment"
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

func TestRetryPolicyForQueueNames(t *testing.T) {
	const (
		worker    = "worker"
		transcode = "transcode"
		audio     = "audio"
	)

	assert.Equal(t, &LooseRetryPolicy, retryPolicyForQueueNames(worker, worker, transcode, audio))
	assert.Equal(t, &StrictRetryPolicy, retryPolicyForQueueNames(transcode, worker, transcode, audio))
	assert.Equal(t, &StrictRetryPolicy, retryPolicyForQueueNames(audio, worker, transcode, audio))

	// The queues the switch never named. They fell through to no policy at all,
	// which leaves the SDK retrying until the schedule-to-close budget runs out.
	assert.Equal(t, &LooseRetryPolicy,
		retryPolicyForQueueNames(environment.QueueLowPriority, worker, transcode, audio))
	assert.Equal(t, &LooseRetryPolicy,
		retryPolicyForQueueNames(environment.QueueLiveIngest, worker, transcode, audio))
}

// With QUEUE=debug every accessor returns the debug queue and one worker runs
// everything, so the ordering of the cases is what decides the answer.
func TestRetryPolicyForQueueNamesUnderDebug(t *testing.T) {
	const debug = "debug"

	assert.Equal(t, &LooseRetryPolicy, retryPolicyForQueueNames(debug, debug, debug, debug))
}

// queueActivity reports the queue it was scheduled on.
type scheduledOn struct {
	Queue               string
	StartToCloseTimeout time.Duration
}

func reportSchedulingActivity(ctx context.Context, _ any) (scheduledOn, error) {
	info := activity.GetInfo(ctx)
	return scheduledOn{
		Queue:               info.TaskQueue,
		StartToCloseTimeout: info.StartToCloseTimeout,
	}, nil
}

func runSchedulingWorkflow(t *testing.T, wf any) scheduledOn {
	t.Helper()

	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(reportSchedulingActivity)

	env.ExecuteWorkflow(wf)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var got scheduledOn
	require.NoError(t, env.GetWorkflowResult(&got))
	return got
}

func executeWorkflow(ctx workflow.Context) (scheduledOn, error) {
	return Execute(ctx, reportSchedulingActivity, any(nil)).Result(ctx)
}

func executeLowPrioWorkflow(ctx workflow.Context) (scheduledOn, error) {
	return ExecuteWithLowPrioQueue(ctx, reportSchedulingActivity, any(nil)).Result(ctx)
}

func TestExecuteSchedulesOnTheQueueThatOwnsTheActivity(t *testing.T) {
	got := runSchedulingWorkflow(t, executeWorkflow)

	assert.Equal(t, environment.GetWorkerQueue(), got.Queue)
	assert.Equal(t, GetDefaultActivityOptions().StartToCloseTimeout, got.StartToCloseTimeout)
}

// The two entry points differ only in the queue they pick; everything the
// workflow did not set has to be filled in the same way for both. It used to
// be filled in only by Execute.
func TestExecuteWithLowPrioQueueMovesTheActivityAndKeepsTheDefaults(t *testing.T) {
	got := runSchedulingWorkflow(t, executeLowPrioWorkflow)

	if environment.GetWorkerQueue() == environment.QueueWorker {
		assert.Equal(t, environment.QueueLowPriority, got.Queue)
	} else {
		// Debug: one worker polls one queue, so nothing is moved off it.
		assert.Equal(t, environment.GetWorkerQueue(), got.Queue)
	}
	assert.Equal(t, GetDefaultActivityOptions().StartToCloseTimeout, got.StartToCloseTimeout)
}
