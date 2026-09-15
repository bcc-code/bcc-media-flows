package miscworkflows

import (
	"fmt"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/emails"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"github.com/bcc-code/bcc-media-flows/services/qscan"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	qscanFastPollInterval = 10 * time.Second
	qscanFastPollFor      = 2 * time.Minute
	qscanPollInterval     = time.Minute
	// qscanTimeout bounds the poll loop so a file QScan never picks up fails
	// with a legible error instead of growing the history until the server
	// terminates the execution.
	qscanTimeout = 8 * time.Hour

	// qscanReportAttempts covers a report that is not written yet when the file
	// turns "analyzed": QScan finishes the analysis before it finishes the PDF.
	qscanReportAttempts = 3

	qscanMaxEmailEvents = 50
)

type QScanMasterInput struct {
	VXID string
	Path paths.Path
	// Recipients are the people who uploaded the master and get the verdict.
	// Errors never go here; they go to the configured QScan error addresses.
	Recipients []string
}

type QScanMasterResult struct {
	Outcome  string
	JobID    int64
	FileID   int64
	Critical int
	Warning  int
	Logging  int
}

// QScanFileInput describes one file to run through QScan.
type QScanFileInput struct {
	VXID string
	Path paths.Path
	// TemplateName selects the QC template; empty means the masters template.
	TemplateName string
	// Description is shown on the job in QScan; empty means the masters wording.
	Description string
}

// QScanFileResult is the verdict for one file, ready to be mailed.
type QScanFileResult struct {
	Report notifications.QScanResult
	// Attachment is the PDF report; nil when none was saved, in which case
	// Report.ReportNote says why.
	Attachment *emails.Attachment
	JobID      int64
	FileID     int64
}

// QScanMaster runs an uploaded master through QScan and emails the result to
// the uploader. It is started as an abandoned child of the master import, so it
// reports its own failures by email: nobody is waiting on its return value.
func QScanMaster(ctx workflow.Context, in QScanMasterInput) (*QScanMasterResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting QScanMaster workflow", "vxid", in.VXID, "path", in.Path.Linux())

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	res, err := runQScanFile(ctx, QScanFileInput{VXID: in.VXID, Path: in.Path})
	if err != nil {
		sendQScanError(ctx, res.Report, err)
		return nil, err
	}

	var attachments []emails.Attachment
	if res.Attachment != nil {
		attachments = append(attachments, *res.Attachment)
	}
	sendQScanReport(ctx, in.Recipients, res.Report, attachments)

	return &QScanMasterResult{
		Outcome:  res.Report.Outcome.Value,
		JobID:    res.JobID,
		FileID:   res.FileID,
		Critical: res.Report.Critical,
		Warning:  res.Report.Warning,
		Logging:  res.Report.Logging,
	}, nil
}

// QScanFile runs one file through QScan and hands the verdict back to the
// parent, which decides who is told. QScanRawImport uses it to batch the files
// of one upload into a single mail.
func QScanFile(ctx workflow.Context, in QScanFileInput) (*QScanFileResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting QScanFile workflow", "vxid", in.VXID, "path", in.Path.Linux(), "template", in.TemplateName)

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())
	return runQScanFile(ctx, in)
}

// runQScanFile queues the file, waits for the analysis and collects the report.
// The result is never nil: on error it carries what the run got as far as
// (job name, status), so the caller can say what failed.
func runQScanFile(ctx workflow.Context, in QScanFileInput) (*QScanFileResult, error) {
	res := &QScanFileResult{
		Report: notifications.QScanResult{
			VXID:     in.VXID,
			Filename: in.Path.Base(),
		},
	}

	job, err := wfutils.Execute(ctx, activities.QScan.QScanEnsureJob, activities.QScanEnsureJobInput{
		VXID:         in.VXID,
		Path:         in.Path,
		TemplateName: in.TemplateName,
		Description:  in.Description,
	}).Result(ctx)
	if err != nil {
		return res, fmt.Errorf("creating QScan job: %w", err)
	}
	res.JobID = job.JobID
	res.Report.JobName = job.JobName
	res.Report.QScanURL = job.JobURL

	file, err := wfutils.Execute(ctx, activities.QScan.QScanEnsureFile, activities.QScanEnsureFileInput{
		JobID: job.JobID,
		Path:  in.Path,
	}).Result(ctx)
	if err != nil {
		return res, fmt.Errorf("queueing file in QScan: %w", err)
	}
	res.FileID = file.FileID

	status, err := waitForQScanFile(ctx, job.JobID, file.FileID)
	if err != nil {
		return res, err
	}
	res.Report.Status = status.Status.String()
	res.Report.StatusInfo = status.StatusInfo

	if !status.Status.IsSuccess() {
		err := fmt.Errorf("QScan did not analyse the file: status %s %s", status.Status, status.StatusInfo)
		return res, temporal.NewNonRetryableApplicationError(err.Error(), "QScanAnalysisFailed", nil)
	}

	tempFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return res, fmt.Errorf("creating temp folder: %w", err)
	}
	reportPath := tempFolder.Append(fmt.Sprintf("%s_qscan_report.pdf", in.VXID))

	fetched, err := fetchQScanResult(ctx, job.JobID, file.FileID, file.ResultsID, reportPath)
	if err != nil {
		return res, fmt.Errorf("fetching QScan result: %w", err)
	}

	res.Report.Critical = int(status.CriticalTotal)
	res.Report.Warning = int(status.WarningTotal)
	res.Report.Logging = int(status.LoggingTotal)
	res.Report.Outcome = qscanOutcome(res.Report.Critical, res.Report.Warning)
	res.Report.Events, res.Report.TotalEvents = selectQScanEvents(fetched.Events, qscanMaxEmailEvents)
	res.Report.ReportNote = fetched.ReportNote

	if fetched.ReportSaved {
		res.Attachment = &emails.Attachment{
			Filename:    reportPath.Base(),
			ContentType: "application/pdf",
			Path:        reportPath,
		}
	}

	return res, nil
}

// waitForQScanFile polls until the file reaches a terminal status. Each pass is
// one activity and one timer, so it polls fast only briefly and then backs off.
func waitForQScanFile(ctx workflow.Context, jobID, fileID int64) (*qscan.FileStatus, error) {
	logger := workflow.GetLogger(ctx)
	start := workflow.Now(ctx)

	for {
		status, err := wfutils.Execute(ctx, activities.QScan.QScanFileStatus, activities.QScanFileStatusInput{
			JobID:  jobID,
			FileID: fileID,
		}).Result(ctx)
		if err != nil {
			return nil, fmt.Errorf("checking QScan status: %w", err)
		}

		if status.Status.IsTerminal() {
			logger.Info("QScan analysis finished", "status", status.Status.String(), "critical", status.CriticalTotal, "warning", status.WarningTotal)
			return status, nil
		}

		waited := workflow.Now(ctx).Sub(start)
		if waited >= qscanTimeout {
			return nil, temporal.NewNonRetryableApplicationError(
				fmt.Sprintf("QScan job %d file %d still %s after %s", jobID, fileID, status.Status, qscanTimeout),
				"QScanTimeout", nil)
		}

		interval := qscanPollInterval
		if waited < qscanFastPollFor {
			interval = qscanFastPollInterval
		}
		if err := workflow.Sleep(ctx, interval); err != nil {
			return nil, err
		}
	}
}

func qscanOutcome(critical, warning int) notifications.QCOutcome {
	switch {
	case critical > 0:
		return notifications.QCFailed
	case warning > 0:
		return notifications.QCPassedWithWarnings
	default:
		return notifications.QCPassed
	}
}

// selectQScanEvents keeps critical events first, then warnings, dropping
// logging events, and returns the total count of everything QScan reported.
func selectQScanEvents(events []qscan.Event, limit int) ([]notifications.QScanEvent, int) {
	var selected []notifications.QScanEvent
	for _, severity := range []string{"critical", "warning"} {
		for _, e := range events {
			if e.Severity != severity || len(selected) >= limit {
				continue
			}
			selected = append(selected, notifications.QScanEvent{
				Severity:  e.Severity,
				MediaType: e.MediaType,
				Message:   e.Message,
				TCIn:      e.TCIn,
				TCOut:     e.TCOut,
			})
		}
	}
	return selected, len(events)
}

// fetchQScanResult asks for the report until it is there. QScan reports a file
// as analysed before the PDF exists, so the first attempt can come too early.
func fetchQScanResult(ctx workflow.Context, jobID, fileID, resultsID int64, reportPath paths.Path) (*activities.QScanFetchResultOutput, error) {
	logger := workflow.GetLogger(ctx)

	for attempt := 1; ; attempt++ {
		fetched, err := wfutils.Execute(ctx, activities.QScan.QScanFetchResult, activities.QScanFetchResultInput{
			JobID:      jobID,
			FileID:     fileID,
			ResultsID:  resultsID,
			ReportPath: reportPath,
		}).Result(ctx)
		if err != nil {
			return nil, err
		}

		if fetched.ReportSaved || attempt == qscanReportAttempts {
			return fetched, nil
		}

		logger.Info("QScan report not available yet, retrying", "jobID", jobID, "fileID", fileID, "attempt", attempt, "note", fetched.ReportNote)
		if err := workflow.Sleep(ctx, qscanPollInterval); err != nil {
			return nil, err
		}
	}
}

// sendQScanError reports a QC that could not be carried out. It goes to the
// configured error addresses rather than to the uploader, who can do nothing
// about a QScan outage.
func sendQScanError(ctx workflow.Context, report notifications.QScanResult, err error) {
	report.Outcome = notifications.QCError
	report.Error = err.Error()

	recipients, sideEffectErr := qscanErrorRecipients(ctx)
	if sideEffectErr != nil {
		workflow.GetLogger(ctx).Error("Failed to resolve the QScan error addresses", "error", sideEffectErr)
		return
	}

	sendQScanReport(ctx, recipients, report, nil)
}

// qscanErrorRecipients reads the configured QC error addresses through a side
// effect, so a config change does not break replay.
func qscanErrorRecipients(ctx workflow.Context) ([]string, error) {
	var recipients []string
	err := workflow.SideEffect(ctx, func(workflow.Context) any {
		return environment.Get().QScan.ErrorEmails()
	}).Get(&recipients)
	return recipients, err
}

func sendQScanReport(ctx workflow.Context, recipients []string, report notifications.Template, attachments []emails.Attachment) {
	logger := workflow.GetLogger(ctx)

	if len(recipients) == 0 {
		logger.Warn("No recipient for the QScan report, sending nothing", "subject", report.Subject())
		return
	}

	email, err := emails.NewMessage(report, recipients, nil, nil)
	if err != nil {
		logger.Error("Failed to build QScan email", "error", err)
		return
	}
	email.Attachments = attachments
	if err := wfutils.Execute(ctx, activities.Util.SendEmail, email).Wait(ctx); err != nil {
		logger.Error("Failed to send QScan email", "error", err)
	}
}
