package miscworkflows

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// grantQScanSlotAtOnce stands in for the QScanLock: every run that asks gets
// the slot straight away, and every release is accepted. It returns the runs
// that released, in order.
func grantQScanSlotAtOnce(env *testsuite.TestWorkflowEnvironment) *[]string {
	env.OnActivity(RequestQScanSlot, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(1).(QScanSlotRequest)
		env.RegisterDelayedCallback(func() {
			if err := env.SignalWorkflowByID(req.WorkflowID, qscanGrantedSignal, nil); err != nil {
				panic(err)
			}
		}, 0)
	}).Return(nil).Maybe()

	var released []string
	env.OnSignalExternalWorkflow(mock.Anything, QScanLockWorkflowID, "", qscanReleaseSignal, mock.Anything).
		Run(func(args mock.Arguments) {
			released = append(released, args.Get(4).(QScanSlotRequest).WorkflowID)
		}).Return(nil).Maybe()
	return &released
}

func req(id string) QScanSlotRequest { return QScanSlotRequest{WorkflowID: id, RunID: id + "-run"} }

// lockGrants records which runs the lock handed the slot to, in order.
func lockGrants(env *testsuite.TestWorkflowEnvironment) *[]string {
	var grants []string
	env.OnSignalExternalWorkflow(mock.Anything, mock.Anything, mock.Anything, qscanGrantedSignal, mock.Anything).
		Run(func(args mock.Arguments) {
			grants = append(grants, args.String(1))
		}).Return(nil)
	return &grants
}

func TestQScanLock_GrantsOneRunAtATimeInOrder(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	grants := lockGrants(env)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(qscanAcquireSignal, req("a"))
		env.SignalWorkflow(qscanAcquireSignal, req("b"))
		// A repeated request does not queue the run twice.
		env.SignalWorkflow(qscanAcquireSignal, req("b"))
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		assert.Equal(t, []string{"a"}, *grants, "b must wait while a holds the slot")
		// Only the holder can release.
		env.SignalWorkflow(qscanReleaseSignal, req("b"))
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		assert.Equal(t, []string{"a"}, *grants)
		env.SignalWorkflow(qscanReleaseSignal, req("a"))
	}, 3*time.Second)
	env.RegisterDelayedCallback(func() {
		assert.Equal(t, []string{"a", "b"}, *grants)
		env.SignalWorkflow(qscanReleaseSignal, req("b"))
	}, 4*time.Second)

	env.ExecuteWorkflow(QScanLock, QScanLockState{})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	assert.Equal(t, []string{"a", "b"}, *grants)
}

func TestQScanLock_TakesTheSlotBackFromARunThatEnded(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	grants := lockGrants(env)

	env.OnActivity(QScanSlotHolderRunning, mock.Anything, req("a")).Return(false, nil).Once()

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(qscanAcquireSignal, req("a"))
		env.SignalWorkflow(qscanAcquireSignal, req("b"))
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(qscanReleaseSignal, req("b"))
	}, qscanLockCheckInterval+time.Minute)

	env.ExecuteWorkflow(QScanLock, QScanLockState{})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	assert.Equal(t, []string{"a", "b"}, *grants)
	env.AssertExpectations(t)
}

func TestQScanLock_SkipsARunThatCannotBeSignalled(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()

	var grants []string
	env.OnSignalExternalWorkflow(mock.Anything, "a", mock.Anything, qscanGrantedSignal, mock.Anything).
		Return(assert.AnError)
	env.OnSignalExternalWorkflow(mock.Anything, "b", mock.Anything, qscanGrantedSignal, mock.Anything).
		Run(func(args mock.Arguments) { grants = append(grants, "b") }).Return(nil)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(qscanAcquireSignal, req("a"))
		env.SignalWorkflow(qscanAcquireSignal, req("b"))
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(qscanReleaseSignal, req("b"))
	}, 2*time.Second)

	env.ExecuteWorkflow(QScanLock, QScanLockState{})

	require.NoError(t, env.GetWorkflowError())
	assert.Equal(t, []string{"b"}, grants)
}

// qscanSlotUser holds the slot for an hour, as a QC run would.
func qscanSlotUser(ctx workflow.Context) error {
	release, err := acquireQScanSlot(ctx)
	if err != nil {
		return err
	}
	defer release()
	return workflow.Sleep(ctx, time.Hour)
}

func TestAcquireQScanSlot_AsksAgainWhileWaiting(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(qscanSlotUser)

	requests := 0
	env.OnActivity(RequestQScanSlot, mock.Anything, mock.Anything).Run(func(mock.Arguments) { requests++ }).Return(nil)
	env.OnSignalExternalWorkflow(mock.Anything, QScanLockWorkflowID, "", qscanReleaseSignal, mock.Anything).Return(nil).Once()

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(qscanGrantedSignal, nil)
	}, qscanSlotRequestRepeat+time.Minute)

	env.ExecuteWorkflow(qscanSlotUser)

	require.NoError(t, env.GetWorkflowError())
	assert.Equal(t, 2, requests)
	env.AssertExpectations(t)
}
