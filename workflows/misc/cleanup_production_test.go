package miscworkflows

import (
	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"testing"
	"time"
)

type CleanupProductionTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *CleanupProductionTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.SetTestTimeout(200 * time.Second)
	s.env.RegisterActivity(cantemo.GetFiles)
	s.env.RegisterActivity(cantemo.GetFormats)
	s.env.RegisterActivity(cantemo.RenameFile)
	s.env.RegisterActivity(MoveFileByImportDateParams{})
}

func (s *CleanupProductionTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CleanupProductionTestSuite) Test_CleanupProduction() {
	s.T().Skip("Not fully implemented")
	s.env.ExecuteWorkflow(SortFilesByImportedDate, SortFilesByImportedDateParams{
		SourceStorageID:      "VX-INVALID",
		DestinationStorageID: "VX-INVALID-2",
		FileList:             []string{"file1", "file2", "file3"},
		BatchSize:            1,
	})
	s.True(s.env.IsWorkflowCompleted())

	err := s.env.GetWorkflowError()
	s.NoError(err)
}

func TestCleanupProduction(t *testing.T) {
	suite.Run(t, new(CleanupProductionTestSuite))
}
