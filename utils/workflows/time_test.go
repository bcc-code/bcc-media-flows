package wfutils

import (
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"testing"
	"time"
)

type UnitTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *UnitTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *UnitTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func NowTestWF(ctx workflow.Context) (time.Time, error) {
	return Now(ctx), nil
}

func (s *UnitTestSuite) Test_SimpleWorkflow_Success() {
	s.env.ExecuteWorkflow(NowTestWF)

	t := &time.Time{}
	err := s.env.GetWorkflowResult(t)

	s.NoError(err)
	s.NotEmpty(t)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func TestUnitTestSuite(t *testing.T) {
	suite.Run(t, new(UnitTestSuite))
}
