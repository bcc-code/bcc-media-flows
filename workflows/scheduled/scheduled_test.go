package scheduled

import (
	"os"
	"testing"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
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

	// Two files per folder, counted per root rather than listed.
	s.NotEmpty(result.DeletedCountPerRoot)
	s.Equal(2*len(result.DeletedCountPerRoot), result.DeletedCount)
	for root, count := range result.DeletedCountPerRoot {
		s.Equal(2, count, "unexpected count for %s", root)
	}
}

func TestScheduledTestSuite(t *testing.T) {
	suite.Run(t, new(ScheduledTestSuite))
}

func (s *ScheduledTestSuite) Test_CleanupTemp_OneCutoffForEveryFolder() {
	var cutoffs []time.Time
	var roots []string

	s.env.OnActivity(activities.Util.DeleteOldFiles, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			input := args.Get(1).(activities.CleanupInput)
			cutoffs = append(cutoffs, input.OlderThan)
			roots = append(roots, input.Root.Local())
		}).Return([]string{}, nil)

	s.env.OnActivity(activities.Util.DeleteEmptyDirectories, mock.Anything, mock.Anything).
		Return(nil, nil)

	start := s.env.Now()

	s.env.ExecuteWorkflow(CleanupTemp)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	s.Len(cutoffs, 57, "one call per folder")
	s.Len(lo.Uniq(roots), 57, "no folder is cleaned twice")

	for _, cutoff := range cutoffs {
		s.Equal(cutoffs[0], cutoff, "no folder currently overrides the default retention")
	}
	s.WithinDuration(start.Add(-defaultRetention), cutoffs[0], time.Minute)
}

func TestCleanupFolder_Retention(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	s := cleanupFolder{Path: "/mnt/temp/"}
	assert.Equal(t, now.Add(-defaultRetention), s.cutoff(now))

	kept := cleanupFolder{Path: "/mnt/isilon/Export", Retention: 90 * 24 * time.Hour}
	assert.Equal(t, now.Add(-90*24*time.Hour), kept.cutoff(now))
	assert.True(t, kept.cutoff(now).Before(s.cutoff(now)),
		"a longer retention sweeps less")
}
