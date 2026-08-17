package wfutils_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type CollectChildResultsTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

type childResult struct {
	ID string
}

type collectProbeParams struct {
	// Fail marks which children return an error.
	Fail []bool
}

// collectProbeResult flattens what CollectChildResults returned. ResultOrError
// carries an error, which the data converter cannot round-trip.
type collectProbeResult struct {
	IDs      []string
	Errors   []string
	Reported int
	Err      string
}

func collectProbeWorkflow(ctx workflow.Context, params collectProbeParams) (collectProbeResult, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1},
	})

	var futures []workflow.Future
	for i, shouldFail := range params.Fail {
		futures = append(futures, wfutils.Execute(ctx, collectProbeActivity, collectProbeInput{
			ID:   fmt.Sprintf("child-%d", i),
			Fail: shouldFail,
		}).Future)
	}

	reported := 0
	results, err := wfutils.CollectChildResults[childResult](ctx, futures, func(error) {
		reported++
	})

	out := collectProbeResult{Reported: reported}
	for _, r := range results {
		id := ""
		if r.Result != nil {
			id = r.Result.ID
		}
		out.IDs = append(out.IDs, id)

		message := ""
		if r.Error != nil {
			message = r.Error.Error()
		}
		out.Errors = append(out.Errors, message)
	}
	if err != nil {
		out.Err = err.Error()
	}
	return out, nil
}

type collectProbeInput struct {
	ID   string
	Fail bool
}

func collectProbeActivity(_ context.Context, input collectProbeInput) (*childResult, error) {
	if input.Fail {
		return nil, fmt.Errorf("%s exploded", input.ID)
	}
	return &childResult{ID: input.ID}, nil
}

func (s *CollectChildResultsTestSuite) run(params collectProbeParams) collectProbeResult {
	env := s.NewTestWorkflowEnvironment()
	env.RegisterActivity(collectProbeActivity)
	env.ExecuteWorkflow(collectProbeWorkflow, params)

	s.Require().True(env.IsWorkflowCompleted())
	s.Require().NoError(env.GetWorkflowError())

	var result collectProbeResult
	s.Require().NoError(env.GetWorkflowResult(&result))
	return result
}

func (s *CollectChildResultsTestSuite) Test_AllSucceed() {
	result := s.run(collectProbeParams{Fail: []bool{false, false, false}})

	s.Empty(result.Err)
	s.Zero(result.Reported)
	s.Equal([]string{"child-0", "child-1", "child-2"}, result.IDs)
	s.Equal([]string{"", "", ""}, result.Errors)
}

func (s *CollectChildResultsTestSuite) Test_AFailureDoesNotDropTheOthers() {
	result := s.run(collectProbeParams{Fail: []bool{false, true, false}})

	s.Len(result.IDs, 3, "one entry per child, failed or not")
	s.Equal("child-0", result.IDs[0])
	s.Contains(result.Errors[1], "child-1 exploded")
	s.Equal("child-2", result.IDs[2], "the child after the failure is still collected")

	s.Equal(1, result.Reported)
	s.Contains(result.Err, "child-1 exploded")
}

func (s *CollectChildResultsTestSuite) Test_EveryFailureIsReportedAndJoined() {
	result := s.run(collectProbeParams{Fail: []bool{true, true}})

	s.Equal(2, result.Reported)
	s.Contains(result.Err, "child-0 exploded")
	s.Contains(result.Err, "child-1 exploded")
}

func (s *CollectChildResultsTestSuite) Test_NoFutures() {
	result := s.run(collectProbeParams{})

	s.Empty(result.IDs)
	s.Empty(result.Err)
}

func TestCollectChildResultsTestSuite(t *testing.T) {
	suite.Run(t, new(CollectChildResultsTestSuite))
}
