package export

import (
	"errors"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

// VODExportTestSuite drives the real VXExportToVOD workflow against mocked
// activities, so the drain fix is covered end to end and not only on the helper.
type VODExportTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func testPath(name string) paths.Path {
	return paths.New(paths.TempDrive, name)
}

// vodTestParams builds a minimal but complete VOD export: one audio language and
// two resolutions, one of which is a downloadable file so both the stream and the
// translated-file branches of onVideoCreated are exercised.
//
// WithChapters, SubtitleFiles and Upload are all left off so the workflow avoids
// the chapter, subtitle-copy and rclone/S3 paths, which are not what this test is
// about. MergeResult.VideoFile is set so it does not take the vizualizer branch.
func vodTestParams() VXExportChildWorkflowParams {
	videoFile := testPath("source.mxf")
	return VXExportChildWorkflowParams{
		RunID: "test-run",
		ParentParams: VXExportParams{
			VXID: "VX-1",
			Resolutions: []utils.Resolution{
				{Width: 1920, Height: 1080, IsFile: false},
				{Width: 960, Height: 540, IsFile: true},
			},
		},
		ExportData: vidispine.ExportData{
			Title:            "Test Export",
			SafeTitle:        "test_export",
			OriginalLanguage: "nor",
		},
		MergeResult: MergeExportDataResult{
			Duration:   60,
			VideoFile:  &videoFile,
			AudioFiles: map[string]paths.Path{"nor": testPath("nor.wav")},
		},
		TempDir:   testPath("temp"),
		OutputDir: testPath("output"),
		Upload:    false,
	}
}

// mockSupportingActivities mocks everything VXExportToVOD needs apart from the
// video transcode, which each test sets up itself.
func (s *VODExportTestSuite) mockSupportingActivities(env *testsuite.TestWorkflowEnvironment) {
	env.OnActivity(activities.Audio.NormalizeAudioActivity, mock.Anything, mock.Anything).Return(
		&activities.NormalizeAudioResult{FilePath: testPath("nor_normalized.wav"), IsSilent: false}, nil).Maybe()

	env.OnActivity(activities.Audio.TranscodeToAudioAac, mock.Anything, mock.Anything).Return(
		&common.AudioResult{OutputPath: testPath("nor.aac")}, nil).Maybe()

	env.OnActivity(activities.Audio.TranscodeMux, mock.Anything, mock.Anything).Return(
		&common.MuxResult{Path: testPath("muxed.mp4")}, nil).Maybe()

	env.OnActivity(activities.Util.WriteFile, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	// notifyExportDone fires only on the success path, and swallows its own errors.
	env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).Return(
		&telegram.Message{}, nil).Maybe()
}

// A failed video transcode must surface as a workflow error. onVideoCreated schedules
// no follow-up futures for a failed video, so any drain that waits on a count derived
// from the inputs blocks until the execution timeout instead of reporting the failure.
func (s *VODExportTestSuite) Test_FailedVideoTranscode_FailsInsteadOfHanging() {
	env := s.NewTestWorkflowEnvironment()
	s.mockSupportingActivities(env)

	// Both resolutions fail, so no follow-up futures are scheduled at all.
	env.OnActivity(activities.Video.TranscodeToVideoH264, mock.Anything, mock.Anything).Return(
		nil, errors.New("ffmpeg exited with code 1"))

	env.ExecuteWorkflow(VXExportToVOD, vodTestParams())

	s.True(env.IsWorkflowCompleted(), "workflow hung instead of completing")
	err := env.GetWorkflowError()
	s.Error(err, "a failed transcode must surface as a workflow error")
	// errors.Join keeps the underlying cause reachable rather than reporting only
	// the first error, or a timeout.
	s.Contains(err.Error(), "ffmpeg exited with code 1")
}

// A partial failure is the case the derived count got most wrong: the successful
// resolution schedules follow-up futures while the failed one schedules none.
func (s *VODExportTestSuite) Test_OneOfTwoVideoTranscodesFails_FailsInsteadOfHanging() {
	env := s.NewTestWorkflowEnvironment()
	s.mockSupportingActivities(env)

	env.OnActivity(activities.Video.TranscodeToVideoH264, mock.Anything, mock.MatchedBy(
		func(input common.VideoInput) bool { return input.Resolution.Height == 1080 },
	)).Return(nil, errors.New("ffmpeg exited with code 1"))

	env.OnActivity(activities.Video.TranscodeToVideoH264, mock.Anything, mock.MatchedBy(
		func(input common.VideoInput) bool { return input.Resolution.Height != 1080 },
	)).Return(&common.VideoResult{OutputPath: testPath("540p.mp4")}, nil)

	env.ExecuteWorkflow(VXExportToVOD, vodTestParams())

	s.True(env.IsWorkflowCompleted(), "workflow hung instead of completing")
	err := env.GetWorkflowError()
	s.Error(err)
	s.Contains(err.Error(), "ffmpeg exited with code 1")
}

// The happy path must still complete, so the drain change is not just failing fast.
func (s *VODExportTestSuite) Test_AllVideoTranscodesSucceed() {
	env := s.NewTestWorkflowEnvironment()
	s.mockSupportingActivities(env)

	env.OnActivity(activities.Video.TranscodeToVideoH264, mock.Anything, mock.Anything).Return(
		&common.VideoResult{OutputPath: testPath("video.mp4")}, nil)

	env.ExecuteWorkflow(VXExportToVOD, vodTestParams())

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result VXExportResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal("VX-1", result.ID)
	s.Equal("aws.smil", result.SmilFile)
}

func TestVODExportTestSuite(t *testing.T) {
	suite.Run(t, new(VODExportTestSuite))
}
