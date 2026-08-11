package ingestworkflows

import (
	"errors"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type SyncFixTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func syncFixAudioPaths() map[string]paths.Path {
	return map[string]paths.Path{
		"nor": paths.New(paths.TempDrive, "nor.wav"),
		"eng": paths.New(paths.TempDrive, "eng.wav"),
	}
}

// mockCommon covers everything up to the copy loop. Adjustment is non-zero in
// every test so the workflow skips the automatic-adjustment branch, which is a
// separate concern from the drain.
func (s *SyncFixTestSuite) mockCommon(env *testsuite.TestWorkflowEnvironment) {
	env.OnActivity(activities.Vidispine.GetRelatedAudioFiles, mock.Anything, mock.Anything).Return(
		syncFixAudioPaths(), nil).Maybe()

	env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).Return(
		&telegram.Message{}, nil).Maybe()

	// Adjustment is positive in these tests, so PrependSilence is the branch taken.
	env.OnActivity(activities.Audio.PrependSilence, mock.Anything, mock.Anything).Return(
		&activities.PrependSilenceOutput{}, nil).Maybe()
	env.OnActivity(activities.Audio.TrimFile, mock.Anything, mock.Anything).Return(
		&activities.TrimResult{}, nil).Maybe()
}

func (s *SyncFixTestSuite) execute(env *testsuite.TestWorkflowEnvironment) error {
	env.ExecuteWorkflow(IngestSyncFix, IngestSyncFixParams{VXID: "VX-1", Adjustment: 100})
	s.True(env.IsWorkflowCompleted(), "workflow hung instead of completing")
	return env.GetWorkflowError()
}

// The happy path: every copy is confirmed and every adjustment applied.
func (s *SyncFixTestSuite) Test_AllLanguagesSucceed() {
	env := s.NewTestWorkflowEnvironment()
	s.mockCommon(env)

	env.OnActivity(activities.Util.RcloneCopyFile, mock.Anything, mock.Anything).Return(1, nil)
	env.OnActivity(activities.Util.RcloneWaitForJob, mock.Anything, mock.Anything).Return(true, nil)

	s.NoError(s.execute(env))
}

// RcloneCopyFile failing means no future is ever registered for that language, so
// the old `2 * len(languages)` drain overshot by two and blocked forever.
func (s *SyncFixTestSuite) Test_CopyFileFails_ReportsErrorInsteadOfHanging() {
	env := s.NewTestWorkflowEnvironment()
	s.mockCommon(env)

	env.OnActivity(activities.Util.RcloneCopyFile, mock.Anything, mock.Anything).Return(
		0, errors.New("rclone refused the copy"))

	err := s.execute(env)
	s.Error(err, "a failed copy must not be reported as success")
	s.Contains(err.Error(), "rclone refused the copy")
}

// The job resolving as "not copied" is the subtler overshoot: the wait future was
// registered, but the callback returns before registering its follow-up, so the
// drain overshot by one.
func (s *SyncFixTestSuite) Test_CopyNotConfirmed_ReportsErrorInsteadOfHanging() {
	env := s.NewTestWorkflowEnvironment()
	s.mockCommon(env)

	env.OnActivity(activities.Util.RcloneCopyFile, mock.Anything, mock.Anything).Return(1, nil)
	env.OnActivity(activities.Util.RcloneWaitForJob, mock.Anything, mock.Anything).Return(false, nil)

	err := s.execute(env)
	s.Error(err)
	s.Contains(err.Error(), "failed to copy file")
}

// A failure in the adjustment itself was also collected and then dropped.
func (s *SyncFixTestSuite) Test_AdjustmentFails_ReportsError() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity(activities.Vidispine.GetRelatedAudioFiles, mock.Anything, mock.Anything).Return(
		syncFixAudioPaths(), nil).Maybe()
	env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).Return(
		&telegram.Message{}, nil).Maybe()
	env.OnActivity(activities.Util.RcloneCopyFile, mock.Anything, mock.Anything).Return(1, nil)
	env.OnActivity(activities.Util.RcloneWaitForJob, mock.Anything, mock.Anything).Return(true, nil)
	env.OnActivity(activities.Audio.PrependSilence, mock.Anything, mock.Anything).Return(
		nil, errors.New("ffmpeg could not prepend silence"))

	err := s.execute(env)
	s.Error(err)
	s.Contains(err.Error(), "ffmpeg could not prepend silence")
}

// Both languages failing must still report both causes, not just the first, and
// must not hang despite the drain overshooting by four.
func (s *SyncFixTestSuite) Test_AllLanguagesFail_ReportsEveryCause() {
	env := s.NewTestWorkflowEnvironment()
	s.mockCommon(env)

	env.OnActivity(activities.Util.RcloneCopyFile, mock.Anything, mock.MatchedBy(
		func(input activities.RcloneFileInput) bool { return input.Source.Base() == "nor.wav" },
	)).Return(0, errors.New("nor copy failed"))

	env.OnActivity(activities.Util.RcloneCopyFile, mock.Anything, mock.MatchedBy(
		func(input activities.RcloneFileInput) bool { return input.Source.Base() != "nor.wav" },
	)).Return(0, errors.New("eng copy failed"))

	err := s.execute(env)
	s.Error(err)
	// errors.Join, not just the first failure.
	s.Contains(err.Error(), "nor copy failed")
	s.Contains(err.Error(), "eng copy failed")
}

func TestSyncFixTestSuite(t *testing.T) {
	suite.Run(t, new(SyncFixTestSuite))
}
