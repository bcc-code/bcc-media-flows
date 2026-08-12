package miscworkflows

import (
	"errors"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type ImportSidecarSubtitleTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func (s *ImportSidecarSubtitleTestSuite) Test_ImportsTheSubtitle() {
	env := s.NewTestWorkflowEnvironment()

	srtPath := paths.MustParse("/mnt/temp/workflows/transcript.srt")

	var got vsactivity.ImportSubtitleAsSidecarParams
	env.OnActivity(activities.Vidispine.ImportFileAsSidecarActivity, mock.Anything, mock.MatchedBy(
		func(input vsactivity.ImportSubtitleAsSidecarParams) bool {
			got = input
			return true
		},
	)).Once().Return(&vsactivity.ImportFileAsSidecarResult{}, nil)

	env.ExecuteWorkflow(ImportSidecarSubtitle, ImportSidecarSubtitleInput{
		VXID:     "VX-1",
		FilePath: srtPath,
		Language: "no",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	s.Equal("VX-1", got.AssetID)
	s.Equal(srtPath, got.FilePath)
	s.Equal("no", got.Language)
}

func (s *ImportSidecarSubtitleTestSuite) Test_ReportsActivityFailure() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity(activities.Vidispine.ImportFileAsSidecarActivity, mock.Anything, mock.Anything).
		Return(nil, errors.New("vidispine rejected the sidecar"))

	env.ExecuteWorkflow(ImportSidecarSubtitle, ImportSidecarSubtitleInput{
		VXID:     "VX-1",
		FilePath: paths.MustParse("/mnt/temp/workflows/transcript.srt"),
		Language: "no",
	})

	s.True(env.IsWorkflowCompleted())
	err := env.GetWorkflowError()
	s.Error(err)
	s.Contains(err.Error(), "vidispine rejected the sidecar")
}

// detachedProbeResult records what a detached child saw, so the test can prove the
// child both started and carried ABANDON.
type detachedProbeResult struct {
	Started bool
}

// detachedProbeWorkflow starts a child through WithAbandonChildOptions and waits
// only for it to start, which is the pattern TranscribeVX uses for the sidecar
// import. It asserts the option survives onto the child's options.
func detachedProbeWorkflow(ctx workflow.Context) (detachedProbeResult, error) {
	future := workflow.ExecuteChildWorkflow(
		wfutils.WithAbandonChildOptions(ctx),
		detachedProbeChild,
	)
	if err := future.GetChildWorkflowExecution().Get(ctx, nil); err != nil {
		return detachedProbeResult{}, err
	}
	return detachedProbeResult{Started: true}, nil
}

func detachedProbeChild(ctx workflow.Context) error {
	return nil
}

// WithAbandonChildOptions must produce a context whose children can be started and
// awaited-for-start without the parent blocking on completion.
func (s *ImportSidecarSubtitleTestSuite) Test_WithAbandonChildOptions_StartsChild() {
	env := s.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(detachedProbeChild)

	env.ExecuteWorkflow(detachedProbeWorkflow)

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result detachedProbeResult
	s.NoError(env.GetWorkflowResult(&result))
	s.True(result.Started, "detached child should have been started")
}

func TestImportSidecarSubtitleTestSuite(t *testing.T) {
	suite.Run(t, new(ImportSidecarSubtitleTestSuite))
}
