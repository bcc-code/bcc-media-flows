package export

import (
	"testing"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// waitForShortJobWorkflow exposes the helper to the test environment, which
// needs a workflow to run activities and timers in.
func waitForShortJobWorkflow(ctx workflow.Context, params activities.CheckJobStatusInput) ([]activities.Keyframe, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1},
	})
	return waitForShortJob(ctx, params)
}

func newShortJobEnv(t *testing.T) *testsuite.TestWorkflowEnvironment {
	t.Helper()
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(waitForShortJobWorkflow)
	return env
}

func TestWaitForShortJobReturnsKeyframesWhenTheJobCompletes(t *testing.T) {
	env := newShortJobEnv(t)

	env.OnActivity(activities.Util.CheckJobStatusActivity, mock.Anything, mock.Anything).
		Return(&activities.GenerateShortRequestResult{Status: "in_progress"}, nil).Twice()
	env.OnActivity(activities.Util.CheckJobStatusActivity, mock.Anything, mock.Anything).
		Return(&activities.GenerateShortRequestResult{
			Status:    "completed",
			Keyframes: []activities.Keyframe{{}},
		}, nil).Once()

	env.ExecuteWorkflow(waitForShortJobWorkflow, activities.CheckJobStatusInput{JobID: "job-1"})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var got []activities.Keyframe
	require.NoError(t, env.GetWorkflowResult(&got))
	assert.Len(t, got, 1)
}

// The loop used to have no exit other than the job finishing, so a job stuck in
// in_progress polled every five seconds until the server terminated the
// execution for exceeding its history limit — roughly 3,600 events an hour with
// nothing in the error to say what happened.
func TestWaitForShortJobGivesUpOnAJobThatNeverFinishes(t *testing.T) {
	env := newShortJobEnv(t)

	polls := 0
	env.OnActivity(activities.Util.CheckJobStatusActivity, mock.Anything, mock.Anything).
		Return(&activities.GenerateShortRequestResult{Status: "in_progress"}, nil).
		Run(func(mock.Arguments) { polls++ })

	env.ExecuteWorkflow(waitForShortJobWorkflow, activities.CheckJobStatusInput{JobID: "stuck-job"})
	require.True(t, env.IsWorkflowCompleted())

	err := env.GetWorkflowError()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stuck-job")
	assert.Contains(t, err.Error(), "still in_progress")

	// Two minutes at 5s plus the rest of the two hours at 30s. The exact number
	// matters less than it being bounded and far below the history limit.
	assert.Less(t, polls, 300, "the backoff should keep the poll count small")
	assert.Greater(t, polls, 20, "but it should still have polled for the full window")
}

func TestWaitForShortJobFailsOnAFailedJob(t *testing.T) {
	env := newShortJobEnv(t)

	env.OnActivity(activities.Util.CheckJobStatusActivity, mock.Anything, mock.Anything).
		Return(&activities.GenerateShortRequestResult{Status: "failed"}, nil).Once()

	env.ExecuteWorkflow(waitForShortJobWorkflow, activities.CheckJobStatusInput{JobID: "job-1"})
	require.True(t, env.IsWorkflowCompleted())

	err := env.GetWorkflowError()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job failed with status: failed")
}
