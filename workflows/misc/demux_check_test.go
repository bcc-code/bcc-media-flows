package miscworkflows

import (
	"errors"
	"strings"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/emails"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

type DemuxCheckTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *DemuxCheckTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *DemuxCheckTestSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
}

var (
	demuxClip1 = paths.New(paths.IsilonDrive, "Production/raw/2026/09/17/run/CLIP_01.mxf")
	demuxClip2 = paths.New(paths.IsilonDrive, "Production/raw/2026/09/17/run/CLIP_02.mxf")
)

func (s *DemuxCheckTestSuite) expectCheck(file paths.Path, result *ffmpeg.DemuxCheckResult, err error) {
	s.env.OnActivity(activities.Video.DemuxCheck, mock.Anything, activities.DemuxCheckParams{FilePath: file}).
		Once().Return(result, err)
}

func (s *DemuxCheckTestSuite) Test_MailsOneReportToTheUploader() {
	s.expectCheck(demuxClip1, &ffmpeg.DemuxCheckResult{ProcessedSeconds: 3, TotalSeconds: 3}, nil)
	s.expectCheck(demuxClip2, &ffmpeg.DemuxCheckResult{
		Errors: 1, Warnings: 2, TotalMessages: 3, ShortRead: true, ProcessedSeconds: 1.8, TotalSeconds: 3,
		Messages: []ffmpeg.DemuxCheckMessage{
			{Level: "warning", Component: "in#0/mxf", Message: "broken or empty index"},
			{Level: "warning", Component: "in#0/mxf", Message: "Packet corrupt"},
			{Level: "error", Component: "demux check", Message: "Reading stopped early"},
		},
	}, nil)

	var sent []emails.Message
	s.env.OnActivity(activities.Util.SendEmail, mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
		sent = append(sent, args.Get(1).(emails.Message))
	}).Return(nil, nil)

	s.env.ExecuteWorkflow(DemuxCheck, DemuxCheckInput{
		Files:      []paths.Path{demuxClip1, demuxClip2},
		Recipients: []string{"uploader@example.com", "Some Name", ""},
	})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var result DemuxCheckResult
	s.NoError(s.env.GetWorkflowResult(&result))
	s.Equal(notifications.QCFailed.Value, result.Outcome)
	s.True(result.Mailed)
	if s.Len(result.Files, 2) {
		s.Equal(notifications.QCPassed.Value, result.Files[0].Outcome)
		s.Equal(notifications.QCFailed.Value, result.Files[1].Outcome)
		s.Nil(result.Files[1].ReportPath)
	}

	if s.Len(sent, 1) {
		s.Equal([]string{"uploader@example.com"}, sent[0].To, "only real addresses are mailed")
		s.Equal("File check FAILED: 2 files", sent[0].Subject)
		s.Contains(sent[0].HTML, "CLIP_02.mxf")
		s.Contains(sent[0].HTML, "Reading stopped early")
	}
	s.env.AssertActivityNotCalled(s.T(), "WriteFile")
}

func (s *DemuxCheckTestSuite) Test_WritesAReportNextToEachFileWithoutRecipients() {
	s.expectCheck(demuxClip1, &ffmpeg.DemuxCheckResult{Warnings: 1, TotalMessages: 1, ProcessedSeconds: 3, TotalSeconds: 3,
		Messages: []ffmpeg.DemuxCheckMessage{{Level: "warning", Component: "aist#0:1/pcm_s16le", Message: "Guessed Channel Layout: mono"}}}, nil)
	// Non-retryable so the test environment does not retry the mock; the real
	// activity's stat error is retried until the mount catches up.
	s.expectCheck(demuxClip2, nil, temporal.NewNonRetryableApplicationError("demux check input: stat: no such file", "Stat", nil))

	var written []activities.WriteFileInput
	s.env.OnActivity(activities.Util.WriteFile, mock.Anything, mock.Anything).Times(2).Run(func(args mock.Arguments) {
		written = append(written, args.Get(1).(activities.WriteFileInput))
	}).Return(nil, nil)

	s.env.ExecuteWorkflow(DemuxCheck, DemuxCheckInput{Files: []paths.Path{demuxClip1, demuxClip2}})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var result DemuxCheckResult
	s.NoError(s.env.GetWorkflowResult(&result))
	s.False(result.Mailed)
	s.Equal(notifications.QCError.Value, result.Outcome, "a check that could not run is the worst outcome")
	if s.Len(result.Files, 2) {
		s.Equal(notifications.QCPassedWithWarnings.Value, result.Files[0].Outcome)
		s.Equal(notifications.QCError.Value, result.Files[1].Outcome)
		if s.NotNil(result.Files[0].ReportPath) {
			s.Equal("CLIP_01.mxf.demux-check.html", result.Files[0].ReportPath.Base())
			s.Equal(demuxClip1.Dir(), result.Files[0].ReportPath.Dir())
		}
	}

	if s.Len(written, 2) {
		s.Equal("CLIP_01.mxf.demux-check.html", written[0].Path.Base())
		s.Contains(string(written[0].Data), "Guessed Channel Layout: mono")
		s.Contains(string(written[0].Data), "PASSED WITH WARNINGS")
		s.NotContains(string(written[0].Data), "CLIP_02.mxf", "each file gets its own report")

		s.Equal("CLIP_02.mxf.demux-check.html", written[1].Path.Base())
		s.Contains(string(written[1].Data), "The check did not run")
		s.Contains(string(written[1].Data), "no such file")
	}
	s.env.AssertActivityNotCalled(s.T(), "SendEmail")
}

func (s *DemuxCheckTestSuite) Test_FailsWhenTheReportCannotBeWritten() {
	s.expectCheck(demuxClip1, &ffmpeg.DemuxCheckResult{}, nil)
	s.env.OnActivity(activities.Util.WriteFile, mock.Anything, mock.Anything).Return(nil, errors.New("read-only file system"))

	s.env.ExecuteWorkflow(DemuxCheck, DemuxCheckInput{Files: []paths.Path{demuxClip1}})
	s.True(s.env.IsWorkflowCompleted())

	err := s.env.GetWorkflowError()
	s.Error(err)
	s.Contains(err.Error(), "writing report")
}

func (s *DemuxCheckTestSuite) Test_NoFiles() {
	s.env.ExecuteWorkflow(DemuxCheck, DemuxCheckInput{Recipients: []string{"a@example.com"}})
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func TestDemuxCheckTestSuite(t *testing.T) {
	suite.Run(t, new(DemuxCheckTestSuite))
}

func TestCapDemuxCheckMessages_KeepsErrorsFirst(t *testing.T) {
	var messages []notifications.DemuxCheckMessage
	for i := 0; i < 10; i++ {
		messages = append(messages, notifications.DemuxCheckMessage{Level: "warning", Message: "w"})
	}
	messages = append(messages, notifications.DemuxCheckMessage{Level: "error", Message: "e"})

	capped := capDemuxCheckMessages([]notifications.DemuxCheckFile{{Messages: messages, TotalMessages: 11}}, 3)
	if assert.Len(t, capped[0].Messages, 3) {
		assert.Equal(t, "error", capped[0].Messages[0].Level)
		assert.Equal(t, "warning", capped[0].Messages[1].Level)
	}
	assert.Equal(t, 11, capped[0].TotalMessages)
	assert.Len(t, messages, 11, "the input is left alone")
}

func TestEmailRecipients(t *testing.T) {
	assert.Equal(t, []string{"a@example.com", "b@example.com"}, emailRecipients([]string{" a@example.com ", "Name Only", "", "b@example.com"}))
	assert.Empty(t, emailRecipients(nil))
	assert.True(t, strings.HasSuffix("x"+DemuxCheckReportSuffix, ".html"))
}
