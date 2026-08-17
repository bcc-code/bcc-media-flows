package wfutils_test

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type IsilonExportFolderTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func isilonExportFolderWorkflow(ctx workflow.Context, safeTitle string) (paths.Path, error) {
	return wfutils.GetIsilonExportFolder(ctx, safeTitle), nil
}

// isilonExportFolderTwiceWorkflow is what VXExport and IsilonExport each did by
// hand, with folder names that disagreed.
func isilonExportFolderTwiceWorkflow(ctx workflow.Context, safeTitle string) ([]paths.Path, error) {
	return []paths.Path{
		wfutils.GetIsilonExportFolder(ctx, safeTitle),
		wfutils.GetIsilonExportFolder(ctx, safeTitle),
	}, nil
}

func (s *IsilonExportFolderTestSuite) Test_NamedAfterTheTitleAndTheRun() {
	env := s.NewTestWorkflowEnvironment()
	env.ExecuteWorkflow(isilonExportFolderWorkflow, "Some Title")

	s.Require().True(env.IsWorkflowCompleted())
	s.Require().NoError(env.GetWorkflowError())

	var got paths.Path
	s.Require().NoError(env.GetWorkflowResult(&got))

	s.Equal(paths.IsilonDrive, got.Drive)
	s.Regexp(`^Export/\d{4}-\d{2}/Some Title-`, got.Path)
}

func (s *IsilonExportFolderTestSuite) Test_TwoCallersAgree() {
	env := s.NewTestWorkflowEnvironment()
	env.ExecuteWorkflow(isilonExportFolderTwiceWorkflow, "Some Title")

	s.Require().True(env.IsWorkflowCompleted())
	s.Require().NoError(env.GetWorkflowError())

	var got []paths.Path
	s.Require().NoError(env.GetWorkflowResult(&got))

	s.Require().Len(got, 2)
	s.Equal(got[0], got[1])
}

func TestIsilonExportFolderTestSuite(t *testing.T) {
	suite.Run(t, new(IsilonExportFolderTestSuite))
}
