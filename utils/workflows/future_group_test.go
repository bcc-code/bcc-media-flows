package wfutils_test

import (
	"testing"
	"time"

	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type FutureGroupTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

// groupProbeParams describes a fan-out shape to push through a FutureGroup.
type groupProbeParams struct {
	// Tasks holds one entry per top-level future. true means its callback
	// registers follow-up work; false means the callback returns early without
	// registering anything, the way real callbacks do when the work they were
	// waiting on failed.
	Tasks []bool

	// FollowUps is how many futures a succeeding callback registers.
	FollowUps int
}

type groupProbeResult struct {
	Callbacks int
	Pending   int
}

// groupProbeWorkflow reproduces the scheduling shape that broke VXExportToVOD and
// IngestSyncFix: the top level registers a future per unit of work, and each
// callback either registers follow-up futures or bails out early.
//
// The property under test is that Wait terminates for every shape. Draining a
// count derived from the inputs instead overshoots as soon as one callback bails,
// and Select then blocks forever on a future that was never registered.
func groupProbeWorkflow(ctx workflow.Context, params groupProbeParams) (groupProbeResult, error) {
	group := wfutils.NewFutureGroup(ctx)
	callbacks := 0

	for _, taskSucceeds := range params.Tasks {
		succeeds := taskSucceeds
		group.Add(workflow.NewTimer(ctx, time.Second), func(workflow.Future) {
			callbacks++
			if !succeeds {
				return
			}
			for i := 0; i < params.FollowUps; i++ {
				group.Add(workflow.NewTimer(ctx, time.Second), func(workflow.Future) {
					callbacks++
				})
			}
		})
	}

	group.Wait(ctx)

	return groupProbeResult{Callbacks: callbacks, Pending: group.Pending()}, nil
}

func (s *FutureGroupTestSuite) run(params groupProbeParams) groupProbeResult {
	env := s.NewTestWorkflowEnvironment()
	env.ExecuteWorkflow(groupProbeWorkflow, params)

	s.True(env.IsWorkflowCompleted(), "workflow did not complete — Wait deadlocked")
	s.NoError(env.GetWorkflowError())

	var result groupProbeResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Zero(result.Pending, "group should be fully drained")
	return result
}

func (s *FutureGroupTestSuite) Test_AllTasksRegisterFollowUps() {
	result := s.run(groupProbeParams{Tasks: []bool{true, true, true}, FollowUps: 2})
	// 3 top-level callbacks + 3×2 follow-ups.
	s.Equal(9, result.Callbacks)
}

// The case a count derived from the inputs gets wrong: one callback bails out
// without registering its follow-ups.
func (s *FutureGroupTestSuite) Test_OneCallbackBailsOut() {
	result := s.run(groupProbeParams{Tasks: []bool{true, false, true}, FollowUps: 2})
	// 3 top-level callbacks + 2×2 follow-ups from the two that continued.
	s.Equal(7, result.Callbacks)
}

func (s *FutureGroupTestSuite) Test_EveryCallbackBailsOut() {
	result := s.run(groupProbeParams{Tasks: []bool{false, false, false}, FollowUps: 2})
	s.Equal(3, result.Callbacks)
}

func (s *FutureGroupTestSuite) Test_NoFollowUpWork() {
	result := s.run(groupProbeParams{Tasks: []bool{true}, FollowUps: 0})
	s.Equal(1, result.Callbacks)
}

// Nothing registered at all must return immediately rather than block.
func (s *FutureGroupTestSuite) Test_NothingRegistered() {
	result := s.run(groupProbeParams{FollowUps: 2})
	s.Equal(0, result.Callbacks)
}

func TestFutureGroupTestSuite(t *testing.T) {
	suite.Run(t, new(FutureGroupTestSuite))
}
