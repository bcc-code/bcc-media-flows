package miscworkflows

import (
	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"testing"
)

type CleanupProductionTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *CleanupProductionTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterActivity(cantemo.GetFiles)
	s.env.RegisterActivity(cantemo.GetFormats)
	s.env.RegisterActivity(cantemo.RenameFile)
}

func (s *CleanupProductionTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CleanupProductionTestSuite) Test_CleanupProduction() {
	s.env.ExecuteWorkflow(CleanupProduction)
	s.True(s.env.IsWorkflowCompleted())

	err := s.env.GetWorkflowError()
	s.NoError(err)
}

func TestCleanupProduction(t *testing.T) {
	suite.Run(t, new(CleanupProductionTestSuite))
}
