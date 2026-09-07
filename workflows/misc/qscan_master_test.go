package miscworkflows

import (
	"errors"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/emails"
	"github.com/bcc-code/bcc-media-flows/services/qscan"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type QScanMasterTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *QScanMasterTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).Maybe().Return(&telegram.Message{}, nil)
}

func (s *QScanMasterTestSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
}

var qscanInput = QScanMasterInput{
	VXID:       "VX-123",
	Path:       paths.New(paths.IsilonDrive, "Production/masters/MASTER_01.mxf"),
	Recipients: []string{"qc@example.com"},
}

var (
	qscanJob  = &activities.QScanJob{JobID: 42, JobName: "VX-123 MASTER_01.mxf", JobURL: "http://qscan"}
	qscanFile = &activities.QScanFile{FileID: 11, ResultsID: 99}
)

func (s *QScanMasterTestSuite) expectSubmission() {
	s.env.OnActivity(activities.QScan.QScanEnsureJob, mock.Anything, activities.QScanEnsureJobInput{
		VXID: "VX-123",
		Path: qscanInput.Path,
	}).Once().Return(qscanJob, nil)
	s.env.OnActivity(activities.QScan.QScanEnsureFile, mock.Anything, activities.QScanEnsureFileInput{
		JobID: 42,
		Path:  qscanInput.Path,
	}).Once().Return(qscanFile, nil)
}

func (s *QScanMasterTestSuite) expectTempFolder() {
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Once().Return(nil, nil)
}

func (s *QScanMasterTestSuite) Test_AnalyzedFileIsEmailedWithReport() {
	s.expectSubmission()

	statusInput := activities.QScanFileStatusInput{JobID: 42, FileID: 11}
	s.env.OnActivity(activities.QScan.QScanFileStatus, mock.Anything, statusInput).Once().Return(&qscan.FileStatus{Status: qscan.StatusQueued}, nil)
	s.env.OnActivity(activities.QScan.QScanFileStatus, mock.Anything, statusInput).Once().Return(&qscan.FileStatus{Status: qscan.StatusAnalyzing, Progress: 50}, nil)
	s.env.OnActivity(activities.QScan.QScanFileStatus, mock.Anything, statusInput).Once().Return(&qscan.FileStatus{
		Status: qscan.StatusAnalyzed, CriticalTotal: 1, WarningTotal: 2, LoggingTotal: 3,
	}, nil)

	s.expectTempFolder()
	var fetchInput activities.QScanFetchResultInput
	s.env.OnActivity(activities.QScan.QScanFetchResult, mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
		fetchInput = args.Get(1).(activities.QScanFetchResultInput)
	}).Return(&activities.QScanFetchResultOutput{
		Events: []qscan.Event{
			{Severity: "logging", MediaType: "format", Message: "Container is MXF"},
			{Severity: "warning", MediaType: "audio", Message: "Mute", TCIn: "00:00:05:00", TCOut: "00:00:06:00"},
			{Severity: "critical", MediaType: "video", Message: "Freeze", TCIn: "00:00:01:00", TCOut: "00:00:03:00"},
		},
		ReportSaved: true,
	}, nil)

	var sent emails.Message
	s.env.OnActivity(activities.Util.SendEmail, mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
		sent = args.Get(1).(emails.Message)
	}).Return(nil, nil)

	s.env.ExecuteWorkflow(QScanMaster, qscanInput)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var result QScanMasterResult
	s.NoError(s.env.GetWorkflowResult(&result))
	s.Equal("FAILED", result.Outcome)
	s.Equal(1, result.Critical)

	s.Equal([]string{"qc@example.com"}, sent.To)
	s.Equal("QC FAILED: VX-123 MASTER_01.mxf", sent.Subject)
	s.Contains(sent.PlainText, "Freeze")
	s.Contains(sent.PlainText, "Mute")
	s.NotContains(sent.PlainText, "Container is MXF")
	s.Contains(sent.HTML, "Events (2 of 3)")
	s.Equal(int64(42), fetchInput.JobID)
	s.Equal(int64(11), fetchInput.FileID)
	s.Equal(int64(99), fetchInput.ResultsID)
	s.Equal("VX-123_qscan_report.pdf", fetchInput.ReportPath.Base())
	if s.Len(sent.Attachments, 1) {
		s.Equal("VX-123_qscan_report.pdf", sent.Attachments[0].Filename)
		s.Equal("application/pdf", sent.Attachments[0].ContentType)
		s.Equal(fetchInput.ReportPath, sent.Attachments[0].Path)
	}
}

func (s *QScanMasterTestSuite) Test_CleanFileIsPassed() {
	s.expectSubmission()
	s.env.OnActivity(activities.QScan.QScanFileStatus, mock.Anything, mock.Anything).Once().Return(&qscan.FileStatus{Status: qscan.StatusAnalyzed}, nil)
	s.expectTempFolder()
	s.env.OnActivity(activities.QScan.QScanFetchResult, mock.Anything, mock.Anything).Once().Return(&activities.QScanFetchResultOutput{
		ReportNote: "The PDF report could not be fetched; open it in QScan.",
	}, nil)

	var sent emails.Message
	s.env.OnActivity(activities.Util.SendEmail, mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
		sent = args.Get(1).(emails.Message)
	}).Return(nil, nil)

	s.env.ExecuteWorkflow(QScanMaster, qscanInput)

	s.NoError(s.env.GetWorkflowError())
	s.Equal("QC PASSED: VX-123 MASTER_01.mxf", sent.Subject)
	s.Empty(sent.Attachments)
	s.Contains(sent.PlainText, "could not be fetched")
}

func (s *QScanMasterTestSuite) Test_SubmitFailureIsEmailedAndFailsTheWorkflow() {
	s.env.OnActivity(activities.QScan.QScanEnsureJob, mock.Anything, mock.Anything).Return(nil, errors.New("qscan POST /jobs failed (status 500): boom"))

	var sent emails.Message
	s.env.OnActivity(activities.Util.SendEmail, mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
		sent = args.Get(1).(emails.Message)
	}).Return(nil, nil)

	s.env.ExecuteWorkflow(QScanMaster, qscanInput)

	s.Error(s.env.GetWorkflowError())
	s.Equal("QC ERROR: VX-123 MASTER_01.mxf", sent.Subject)
	s.Contains(sent.PlainText, "boom")
}

func (s *QScanMasterTestSuite) Test_FileErrorIsReportedWithoutFetchingAReport() {
	s.expectSubmission()
	s.env.OnActivity(activities.QScan.QScanFileStatus, mock.Anything, mock.Anything).Once().Return(&qscan.FileStatus{
		Status: qscan.StatusFileError, StatusInfo: "File not found",
	}, nil)

	var sent emails.Message
	s.env.OnActivity(activities.Util.SendEmail, mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
		sent = args.Get(1).(emails.Message)
	}).Return(nil, nil)

	s.env.ExecuteWorkflow(QScanMaster, qscanInput)

	s.Error(s.env.GetWorkflowError())
	s.Equal("QC ERROR: VX-123 MASTER_01.mxf", sent.Subject)
	s.Contains(sent.PlainText, "file_error")
	s.Contains(sent.PlainText, "File not found")
}

func TestQScanMasterTestSuite(t *testing.T) {
	suite.Run(t, new(QScanMasterTestSuite))
}
