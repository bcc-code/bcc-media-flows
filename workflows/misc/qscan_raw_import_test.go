package miscworkflows

import (
	"errors"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/emails"
	"github.com/bcc-code/bcc-media-flows/services/qscan"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type QScanRawImportTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *QScanRawImportTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	// The per-file children really run, so the suite covers the whole chain.
	s.env.RegisterWorkflow(QScanFile)
}

func (s *QScanRawImportTestSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
	s.env.AssertActivityNotCalled(s.T(), "SendTelegramMessage")
}

func (s *QScanRawImportTestSuite) useErrorAddresses(addresses string) {
	s.T().Setenv("QSCAN_ERROR_EMAILS", addresses)
	environment.Load()
	s.T().Cleanup(func() { environment.Load() })
}

var (
	rawClip1 = QScanRawImportFile{VXID: "VX-1", Path: paths.New(paths.IsilonDrive, "Production/raw/2026/09/15/run/CLIP_01.mxf")}
	rawClip2 = QScanRawImportFile{VXID: "VX-2", Path: paths.New(paths.IsilonDrive, "Production/raw/2026/09/15/run/CLIP_02.mxf")}

	rawImportInput = QScanRawImportInput{
		Files:      []QScanRawImportFile{rawClip1, rawClip2},
		Recipients: []string{"uploader@example.com"},
	}
)

// expectSubmission queues one clip in QScan. The job id is the file's index
// plus 40, the file id its index plus 10.
func (s *QScanRawImportTestSuite) expectSubmission(clip QScanRawImportFile, jobID, fileID int64) {
	s.env.OnActivity(activities.QScan.QScanEnsureJob, mock.Anything, activities.QScanEnsureJobInput{
		VXID:         clip.VXID,
		Path:         clip.Path,
		TemplateName: qscanRawImportTemplate,
		Description:  "Automatic QC of raw import " + clip.VXID,
	}).Once().Return(&activities.QScanJob{JobID: jobID, JobName: clip.VXID + " " + clip.Path.Base(), JobURL: "http://qscan"}, nil)
	s.env.OnActivity(activities.QScan.QScanEnsureFile, mock.Anything, activities.QScanEnsureFileInput{
		JobID: jobID,
		Path:  clip.Path,
	}).Once().Return(&activities.QScanFile{FileID: fileID}, nil)
}

func (s *QScanRawImportTestSuite) expectAnalysed(jobID, fileID int64, status *qscan.FileStatus, fetched *activities.QScanFetchResultOutput) {
	s.env.OnActivity(activities.QScan.QScanFileStatus, mock.Anything, activities.QScanFileStatusInput{JobID: jobID, FileID: fileID}).
		Once().Return(status, nil)
	s.env.OnActivity(activities.QScan.QScanFetchResult, mock.Anything, mock.MatchedBy(func(in activities.QScanFetchResultInput) bool {
		return in.JobID == jobID && in.FileID == fileID
	})).Once().Return(fetched, nil)
}

func (s *QScanRawImportTestSuite) captureEmails() *[]emails.Message {
	var sent []emails.Message
	s.env.OnActivity(activities.Util.SendEmail, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		sent = append(sent, args.Get(1).(emails.Message))
	}).Return(nil, nil)
	return &sent
}

func (s *QScanRawImportTestSuite) Test_AllFilesReportedInOneEmail() {
	s.expectSubmission(rawClip1, 41, 11)
	s.expectSubmission(rawClip2, 42, 12)
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Times(2).Return(nil, nil)

	s.expectAnalysed(41, 11, &qscan.FileStatus{Status: qscan.StatusAnalyzed}, &activities.QScanFetchResultOutput{ReportSaved: true})
	s.expectAnalysed(42, 12, &qscan.FileStatus{Status: qscan.StatusAnalyzed, CriticalTotal: 1}, &activities.QScanFetchResultOutput{
		Events:      []qscan.Event{{Severity: "critical", MediaType: "video", Message: "Freeze", TCIn: "00:00:01:00"}},
		ReportSaved: true,
	})

	sent := s.captureEmails()

	s.env.ExecuteWorkflow(QScanRawImport, rawImportInput)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var result QScanRawImportResult
	s.NoError(s.env.GetWorkflowResult(&result))
	s.Equal("FAILED", result.Outcome)
	s.Equal(2, result.Analysed)
	s.Equal(0, result.Failed)
	s.Len(result.Files, 2)

	if !s.Len(*sent, 1, "one upload, one mail") {
		return
	}
	mail := (*sent)[0]
	s.Equal([]string{"uploader@example.com"}, mail.To)
	s.Equal("QC FAILED: raw import, 2 files", mail.Subject)
	s.Contains(mail.PlainText, "VX-1")
	s.Contains(mail.PlainText, "VX-2")
	s.Contains(mail.PlainText, "Freeze")
	s.Contains(mail.HTML, "CLIP_01.mxf")
	s.Contains(mail.HTML, "CLIP_02.mxf")
	if s.Len(mail.Attachments, 2) {
		s.Equal("VX-1_qscan_report.pdf", mail.Attachments[0].Filename)
		s.Equal("VX-2_qscan_report.pdf", mail.Attachments[1].Filename)
		s.Equal("application/pdf", mail.Attachments[0].ContentType)
	}
}

func (s *QScanRawImportTestSuite) Test_FailedFileGoesToOpsAndTheRestToTheUploader() {
	s.useErrorAddresses("ops@example.com")
	s.expectSubmission(rawClip1, 41, 11)
	s.expectSubmission(rawClip2, 42, 12)
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Once().Return(nil, nil)

	s.env.OnActivity(activities.QScan.QScanFileStatus, mock.Anything, activities.QScanFileStatusInput{JobID: 41, FileID: 11}).
		Once().Return(&qscan.FileStatus{Status: qscan.StatusFileError, StatusInfo: "File not found"}, nil)
	s.expectAnalysed(42, 12, &qscan.FileStatus{Status: qscan.StatusAnalyzed}, &activities.QScanFetchResultOutput{ReportSaved: true})

	sent := s.captureEmails()

	s.env.ExecuteWorkflow(QScanRawImport, rawImportInput)

	s.NoError(s.env.GetWorkflowError())
	var result QScanRawImportResult
	s.NoError(s.env.GetWorkflowResult(&result))
	s.Equal(1, result.Analysed)
	s.Equal(1, result.Failed)
	s.Equal("PASSED", result.Outcome)

	if !s.Len(*sent, 2) {
		return
	}
	var ops, uploader *emails.Message
	for i := range *sent {
		switch (*sent)[i].To[0] {
		case "ops@example.com":
			ops = &(*sent)[i]
		case "uploader@example.com":
			uploader = &(*sent)[i]
		}
	}
	if s.NotNil(ops, "the failure goes to the error addresses") {
		s.Equal("QC ERROR: raw import, 1 file", ops.Subject)
		s.Contains(ops.PlainText, "VX-1")
		s.Contains(ops.PlainText, "file_error")
		s.Contains(ops.PlainText, "File not found")
		s.NotContains(ops.PlainText, "child workflow execution error")
		s.Empty(ops.Attachments)
	}
	if s.NotNil(uploader, "the verdict goes to the uploader") {
		s.Equal("QC PASSED: raw import, 1 file", uploader.Subject)
		s.Contains(uploader.PlainText, "VX-2")
		s.NotContains(uploader.PlainText, "VX-1")
		s.Len(uploader.Attachments, 1)
	}
}

func (s *QScanRawImportTestSuite) Test_NothingIsSentWhenThereIsNoUploaderToTell() {
	s.expectSubmission(rawClip1, 41, 11)
	s.expectSubmission(rawClip2, 42, 12)
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Times(2).Return(nil, nil)
	s.expectAnalysed(41, 11, &qscan.FileStatus{Status: qscan.StatusAnalyzed}, &activities.QScanFetchResultOutput{ReportSaved: true})
	s.expectAnalysed(42, 12, &qscan.FileStatus{Status: qscan.StatusAnalyzed}, &activities.QScanFetchResultOutput{ReportSaved: true})

	input := rawImportInput
	input.Recipients = nil
	s.env.ExecuteWorkflow(QScanRawImport, input)

	s.NoError(s.env.GetWorkflowError())
	var result QScanRawImportResult
	s.NoError(s.env.GetWorkflowResult(&result))
	s.Equal(2, result.Analysed)
	s.env.AssertActivityNotCalled(s.T(), "SendEmail")
}

func (s *QScanRawImportTestSuite) Test_AllFilesFailingFailsTheWorkflow() {
	s.useErrorAddresses("ops@example.com")
	s.env.OnActivity(activities.QScan.QScanEnsureJob, mock.Anything, mock.Anything).
		Return(nil, errors.New("qscan POST /jobs failed (status 500): boom"))

	sent := s.captureEmails()

	s.env.ExecuteWorkflow(QScanRawImport, rawImportInput)

	s.Error(s.env.GetWorkflowError())
	if s.Len(*sent, 1) {
		mail := (*sent)[0]
		s.Equal([]string{"ops@example.com"}, mail.To)
		s.Equal("QC ERROR: raw import, 2 files", mail.Subject)
		s.Contains(mail.PlainText, "VX-1")
		s.Contains(mail.PlainText, "VX-2")
		s.Contains(mail.PlainText, "boom")
	}
}

func (s *QScanRawImportTestSuite) Test_NoFilesIsAnError() {
	s.env.ExecuteWorkflow(QScanRawImport, QScanRawImportInput{Recipients: []string{"uploader@example.com"}})

	s.Error(s.env.GetWorkflowError())
	s.env.AssertActivityNotCalled(s.T(), "SendEmail")
}

func TestQScanRawImportTestSuite(t *testing.T) {
	suite.Run(t, new(QScanRawImportTestSuite))
}
