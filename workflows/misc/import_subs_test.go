package miscworkflows

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type ImportSubsTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func (s *ImportSubsTestSuite) mockSubtrans(env *testsuite.TestWorkflowEnvironment, vxid string, subs map[string]paths.Path) {
	env.OnActivity(activities.Util.GetSubtransIDActivity, mock.Anything, activities.GetSubtransIDInput{
		VXID:     vxid,
		NoSubsOK: true,
	}).Return(&activities.GetSubtransIDOutput{SubtransID: "story-1"}, nil)
	env.OnActivity(activities.Util.GetSubtitlesActivity, mock.Anything, mock.Anything).Return(subs, nil)
	env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
}

// Cleanup of stl_subtitle runs exactly once even with multiple languages, and
// every language gets both a shape and a sidecar import.
func (s *ImportSubsTestSuite) Test_CleansOnceForMultipleLanguages() {
	env := s.NewTestWorkflowEnvironment()
	vxid := "VX-42"

	subs := map[string]paths.Path{
		"nor": paths.MustParse("/mnt/temp/subs/nor.srt"),
		"deu": paths.MustParse("/mnt/temp/subs/deu.srt"),
	}
	s.mockSubtrans(env, vxid, subs)

	env.OnActivity(activities.Vidispine.ImportFileAsShapeActivity, mock.Anything, mock.Anything).
		Return(&vsactivity.ImportFileResult{JobID: "job-shape"}, nil).Times(2)
	env.OnActivity(activities.Vidispine.DeleteMetadataGroupInstancesActivity, mock.Anything, vsactivity.DeleteMetadataGroupParams{
		VXID:  vxid,
		Group: "stl_subtitle",
	}).Return(&vsactivity.DeleteMetadataGroupResult{DeletedInstances: 3}, nil).Once()
	env.OnActivity(activities.Vidispine.ImportFileAsSidecarActivity, mock.Anything, mock.Anything).
		Return(&vsactivity.ImportFileAsSidecarResult{JobID: "job-sidecar"}, nil).Times(2)
	env.OnActivity(activities.Vidispine.WaitForJobCompletion, mock.Anything, mock.Anything).
		Return(nil, nil).Times(4)

	env.ExecuteWorkflow(ImportSubtitlesFromSubtrans, ImportSubtitlesFromSubtransInput{VXID: vxid})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}

// A failing sidecar job now fails the workflow instead of being discarded.
func (s *ImportSubsTestSuite) Test_SidecarJobFailureFailsWorkflow() {
	env := s.NewTestWorkflowEnvironment()
	vxid := "VX-42"

	subs := map[string]paths.Path{
		"nor": paths.MustParse("/mnt/temp/subs/nor.srt"),
	}
	s.mockSubtrans(env, vxid, subs)

	env.OnActivity(activities.Vidispine.ImportFileAsShapeActivity, mock.Anything, mock.Anything).
		Return(&vsactivity.ImportFileResult{JobID: "job-shape"}, nil)
	env.OnActivity(activities.Vidispine.DeleteMetadataGroupInstancesActivity, mock.Anything, mock.Anything).
		Return(&vsactivity.DeleteMetadataGroupResult{}, nil)
	env.OnActivity(activities.Vidispine.ImportFileAsSidecarActivity, mock.Anything, mock.Anything).
		Return(&vsactivity.ImportFileAsSidecarResult{JobID: "job-sidecar"}, nil)
	env.OnActivity(activities.Vidispine.WaitForJobCompletion, mock.Anything, vsactivity.WaitForJobCompletionParams{JobID: "job-shape", SleepTime: 10}).
		Return(nil, nil)
	env.OnActivity(activities.Vidispine.WaitForJobCompletion, mock.Anything, vsactivity.WaitForJobCompletionParams{JobID: "job-sidecar", SleepTime: 10}).
		Return(nil, assert.AnError)

	env.ExecuteWorkflow(ImportSubtitlesFromSubtrans, ImportSubtitlesFromSubtransInput{VXID: vxid})

	s.True(env.IsWorkflowCompleted())
	err := env.GetWorkflowError()
	s.Error(err)
	s.Contains(err.Error(), "sidecar import job")
}

func TestImportSubsTestSuite(t *testing.T) {
	suite.Run(t, new(ImportSubsTestSuite))
}
