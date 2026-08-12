package export

import (
	"errors"
	"testing"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type VODDrainTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

// drainProbeParams describes a fan-out shape to push through
// vxExportVodService.addFuture and waitForFiles.
type drainProbeParams struct {
	// Videos holds one entry per simulated video transcode. true means the
	// callback schedules follow-up work; false means it returns early without
	// scheduling anything, the way onVideoCreated does when a transcode fails or
	// when no language is found for the resolution.
	Videos []bool

	// FollowUps is how many futures a succeeding callback schedules, standing in
	// for the stream file plus one translated file per audio language.
	FollowUps int
}

type drainProbeResult struct {
	Callbacks int
	Pending   int
}

// drainProbeWorkflow reproduces the scheduling shape of VXExportToVOD: the top
// level registers one future per video, and each video callback either registers
// follow-up futures or bails out early.
//
// The point of the test is that waitForFiles must terminate for every shape. The
// original code drained a count derived from the input lists
// (qualitiesWithLanguages, and Resolutions × audioKeys), which overshoots as soon
// as any callback returns early — Select then blocked forever on a future that was
// never scheduled.
func drainProbeWorkflow(ctx workflow.Context, params drainProbeParams) (drainProbeResult, error) {
	service := &vxExportVodService{filesSelector: workflow.NewSelector(ctx)}
	callbacks := 0

	for _, videoSucceeds := range params.Videos {
		succeeds := videoSucceeds
		service.addFuture(workflow.NewTimer(ctx, time.Second), func(workflow.Future) {
			callbacks++
			if !succeeds {
				return
			}
			for i := 0; i < params.FollowUps; i++ {
				service.addFuture(workflow.NewTimer(ctx, time.Second), func(workflow.Future) {
					callbacks++
				})
			}
		})
	}

	service.waitForFiles(ctx)

	return drainProbeResult{Callbacks: callbacks, Pending: service.pendingFiles}, nil
}

func (s *VODDrainTestSuite) run(params drainProbeParams) drainProbeResult {
	env := s.NewTestWorkflowEnvironment()
	env.ExecuteWorkflow(drainProbeWorkflow, params)

	// The assertion that matters: the drain terminates for every shape.
	s.True(env.IsWorkflowCompleted(), "workflow did not complete — waitForFiles deadlocked")
	s.NoError(env.GetWorkflowError())

	var result drainProbeResult
	s.NoError(env.GetWorkflowResult(&result))
	s.Zero(result.Pending, "pending count should be fully drained")
	return result
}

// Every video succeeds: the baseline, where the drain count and the number of
// scheduled futures happen to agree.
func (s *VODDrainTestSuite) Test_AllVideosSucceed() {
	result := s.run(drainProbeParams{Videos: []bool{true, true, true}, FollowUps: 2})
	// 3 video callbacks + 3×2 follow-ups.
	s.Equal(9, result.Callbacks)
}

// The regression: one failed transcode schedules no follow-ups, so a count taken
// from the input lists overshoots by FollowUps and Select blocks forever.
func (s *VODDrainTestSuite) Test_OneVideoFails_DoesNotDeadlock() {
	result := s.run(drainProbeParams{Videos: []bool{true, false, true}, FollowUps: 2})
	// 3 video callbacks + 2×2 follow-ups from the two that succeeded.
	s.Equal(7, result.Callbacks)
}

func (s *VODDrainTestSuite) Test_AllVideosFail_DoesNotDeadlock() {
	result := s.run(drainProbeParams{Videos: []bool{false, false, false}, FollowUps: 2})
	s.Equal(3, result.Callbacks)
}

// A single video with no follow-up work, i.e. no IsFile resolutions and no
// languages assigned.
func (s *VODDrainTestSuite) Test_NoFollowUpWork() {
	result := s.run(drainProbeParams{Videos: []bool{true}, FollowUps: 0})
	s.Equal(1, result.Callbacks)
}

// Nothing scheduled at all must not block either.
func (s *VODDrainTestSuite) Test_NoVideos() {
	result := s.run(drainProbeParams{FollowUps: 2})
	s.Equal(0, result.Callbacks)
}

func TestVODDrainTestSuite(t *testing.T) {
	suite.Run(t, new(VODDrainTestSuite))
}

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
