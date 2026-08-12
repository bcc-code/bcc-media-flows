package export

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type PlayoutExportTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func audioOnlyParams() VXExportChildWorkflowParams {
	return VXExportChildWorkflowParams{
		RunID:        "test-run",
		ParentParams: VXExportParams{VXID: "VX-1", Resolutions: []utils.Resolution{{Width: 1920, Height: 1080}}},
		ExportData:   vidispine.ExportData{Title: "Audio Only", SafeTitle: "audio_only"},
		MergeResult: MergeExportDataResult{
			Duration: 60,
			// VXExport sets MakeVideo from fileInfo.HasVideo, so this is nil for
			// audio-only items.
			VideoFile:  nil,
			AudioFiles: map[string]paths.Path{"nor": paths.New(paths.TempDrive, "nor.wav")},
		},
		TempDir:   paths.New(paths.TempDrive, "temp"),
		OutputDir: paths.New(paths.TempDrive, "output"),
	}
}

// Dereferencing a nil VideoFile panicked the workflow task, and Temporal retries a
// panicking task indefinitely — so the export never failed, it just span. It must
// fail once, with a message that says why.
func (s *PlayoutExportTestSuite) Test_AudioOnlyItem_FailsWithoutPanicking() {
	env := s.NewTestWorkflowEnvironment()
	// Mocked so that, without the guard, execution reaches the
	// *params.MergeResult.VideoFile dereference rather than stopping earlier on an
	// unmocked activity.
	env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	env.ExecuteWorkflow(VXExportToXDCAM, audioOnlyParams())

	s.True(env.IsWorkflowCompleted(), "workflow should have completed, not retried forever")

	err := env.GetWorkflowError()
	s.Require().Error(err)
	s.Contains(err.Error(), "audio-only")
}

// The guard must not be retried: no amount of retrying will produce a video file.
func (s *PlayoutExportTestSuite) Test_AudioOnlyItem_IsNotRetried() {
	env := s.NewTestWorkflowEnvironment()
	// Mocked so that, without the guard, execution reaches the
	// *params.MergeResult.VideoFile dereference rather than stopping earlier on an
	// unmocked activity.
	env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	env.ExecuteWorkflow(VXExportToXDCAM, audioOnlyParams())

	s.True(env.IsWorkflowCompleted())
	err := env.GetWorkflowError()
	s.Require().Error(err)

	// A non-retryable application error carries its type through to the caller.
	s.Contains(err.Error(), "NO_VIDEO_FILE")
}

func TestPlayoutExportTestSuite(t *testing.T) {
	suite.Run(t, new(PlayoutExportTestSuite))
}
