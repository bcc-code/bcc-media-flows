package scheduled

import (
	"os"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type ScheduledTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *ScheduledTestSuite) SetupTest() {
	os.Setenv("TEMPORAL_DEBUG", "true")
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *ScheduledTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *ScheduledTestSuite) Test_MediabankenPurgeTrash() {
	trashedIDs := []string{"VX-100", "VX-200", "VX-300"}

	s.env.OnActivity(activities.Vidispine.GetTrashedItems, mock.Anything, nil).
		Return(trashedIDs, nil)

	s.env.OnActivity(activities.Vidispine.DeleteItems, mock.Anything, vsactivity.DeleteItemsParams{
		VXIDs:       trashedIDs,
		DeleteFiles: true,
	}).Return(nil, nil)

	s.env.ExecuteWorkflow(MediabankenPurgeTrash)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.NoError(err)

	var result MediabankenPurgeTrashResult
	s.env.GetWorkflowResult(&result)
	s.Equal(trashedIDs, result.DeletedVXIDs)
}

func (s *ScheduledTestSuite) Test_MediabankenPurgeTrash_Empty() {
	s.env.OnActivity(activities.Vidispine.GetTrashedItems, mock.Anything, nil).
		Return([]string{}, nil)

	s.env.OnActivity(activities.Vidispine.DeleteItems, mock.Anything, vsactivity.DeleteItemsParams{
		VXIDs:       []string{},
		DeleteFiles: true,
	}).Return(nil, nil)

	s.env.ExecuteWorkflow(MediabankenPurgeTrash)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.NoError(err)

	var result MediabankenPurgeTrashResult
	s.env.GetWorkflowResult(&result)
	s.Empty(result.DeletedVXIDs)
}

func (s *ScheduledTestSuite) Test_CleanupTemp() {
	s.env.OnActivity(activities.Util.DeleteOldFiles, mock.Anything, mock.Anything).
		Return([]string{"file1.tmp", "file2.tmp"}, nil)

	s.env.OnActivity(activities.Util.DeleteEmptyDirectories, mock.Anything, mock.Anything).
		Return(nil, nil)

	s.env.ExecuteWorkflow(CleanupTemp)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.NoError(err)

	var result CleanupResult
	s.env.GetWorkflowResult(&result)
	s.Greater(result.DeletedCount, 0)
}

func TestScheduledTestSuite(t *testing.T) {
	suite.Run(t, new(ScheduledTestSuite))
}
