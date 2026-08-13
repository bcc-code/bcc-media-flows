package ingestworkflows

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// waitForTransferSignalWorkflow reports whether the signal arrived and how long
// the wait took on the workflow clock.
type signalWaitResult struct {
	Received bool
	Waited   time.Duration
}

func waitForTransferSignalWorkflow(ctx workflow.Context, expectedFilename string) (signalWaitResult, error) {
	start := workflow.Now(ctx)
	received := waitForTransferSignal(
		ctx,
		workflow.GetSignalChannel(ctx, FileTransferredSignal),
		expectedFilename,
		copyRetryInterval,
	)
	return signalWaitResult{Received: received, Waited: workflow.Now(ctx).Sub(start)}, nil
}

func runSignalWait(t *testing.T, setup func(env *testsuite.TestWorkflowEnvironment)) signalWaitResult {
	t.Helper()

	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(waitForTransferSignalWorkflow)
	setup(env)

	env.ExecuteWorkflow(waitForTransferSignalWorkflow, "growing.mxf")
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var res signalWaitResult
	require.NoError(t, env.GetWorkflowResult(&res))
	return res
}

// The signal used to be read only after the next copy returned, so a transfer
// that finished early still cost a full retry cycle of rsync on a file nobody
// was writing to any more.
func TestWaitForTransferSignalReturnsAsSoonAsTheSignalArrives(t *testing.T) {
	res := runSignalWait(t, func(env *testsuite.TestWorkflowEnvironment) {
		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(FileTransferredSignal, "/mnt/somewhere/growing.mxf")
		}, 5*time.Second)
	})

	assert.True(t, res.Received)
	assert.Equal(t, 5*time.Second, res.Waited, "it should not have waited out the retry interval")
}

func TestWaitForTransferSignalWaitsOutTheIntervalWhenNoSignalArrives(t *testing.T) {
	res := runSignalWait(t, func(env *testsuite.TestWorkflowEnvironment) {})

	assert.False(t, res.Received)
	assert.Equal(t, copyRetryInterval, res.Waited)
}

// Signals name whichever file finished, so the ingest has to ignore the ones
// that are not its own rather than treat any signal as completion.
func TestWaitForTransferSignalIgnoresOtherFiles(t *testing.T) {
	res := runSignalWait(t, func(env *testsuite.TestWorkflowEnvironment) {
		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(FileTransferredSignal, "/mnt/somewhere/other.mxf")
		}, 5*time.Second)
	})

	assert.False(t, res.Received)
	assert.Equal(t, copyRetryInterval, res.Waited, "an unrelated signal must not shorten the wait")
}

// A signal sent while the copy activity was running is queued on the channel,
// so the next wait has to see it without sleeping at all.
func TestWaitForTransferSignalSeesASignalQueuedBeforeItStarts(t *testing.T) {
	res := runSignalWait(t, func(env *testsuite.TestWorkflowEnvironment) {
		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(FileTransferredSignal, "growing.mxf")
		}, 0)
	})

	assert.True(t, res.Received)
	assert.Zero(t, res.Waited)
}
