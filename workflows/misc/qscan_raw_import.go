package miscworkflows

import (
	"errors"
	"fmt"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/emails"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// qscanRawImportTemplate is the QScan template raw material is checked against.
// It has to exist on the QScan server under exactly this name.
const qscanRawImportTemplate = "BCCM - Raw Import QC"

type QScanRawImportFile struct {
	VXID string
	Path paths.Path
}

type QScanRawImportInput struct {
	Files []QScanRawImportFile
	// Recipients are the people who uploaded the files and get one mail with
	// every verdict. With none the QC still runs, but nothing is mailed.
	Recipients []string
}

type QScanRawImportResult struct {
	// Outcome is the worst verdict among the analysed files.
	Outcome  string
	Analysed int
	// Failed counts files QScan could not analyse; those are reported to the
	// configured error addresses, not to the uploader.
	Failed int
	Files  []QScanMasterResult
}

// QScanRawImport runs every video file of one raw import through QScan and
// mails the uploader a single report for the whole upload. It is started as an
// abandoned child of the import, so it reports its own failures by email.
//
// Each file is a child workflow of its own: the QScan poll loop can run for
// hours, and keeping every loop in its own history keeps this one short.
func QScanRawImport(ctx workflow.Context, in QScanRawImportInput) (*QScanRawImportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting QScanRawImport workflow", "files", len(in.Files), "recipients", len(in.Recipients))

	if len(in.Files) == 0 {
		return nil, errors.New("no files to QC")
	}

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	futures := make([]workflow.ChildWorkflowFuture, len(in.Files))
	for i, f := range in.Files {
		futures[i] = workflow.ExecuteChildWorkflow(wfutils.WithChildSearchAttributes(ctx, f.VXID), QScanFile, QScanFileInput{
			VXID:         f.VXID,
			Path:         f.Path,
			TemplateName: qscanRawImportTemplate,
			Description:  "Automatic QC of raw import " + f.VXID,
		})
	}

	var analysed, failed []notifications.QScanResult
	var attachments []emails.Attachment
	var errs []error
	result := &QScanRawImportResult{}

	for i, future := range futures {
		var res QScanFileResult
		if err := future.Get(ctx, &res); err != nil {
			logger.Error("QScan failed for a raw import file", "vxid", in.Files[i].VXID, "error", err)
			errs = append(errs, err)
			failed = append(failed, notifications.QScanResult{
				VXID:     in.Files[i].VXID,
				Filename: in.Files[i].Path.Base(),
				Outcome:  notifications.QCError,
				Error:    qscanChildErrorMessage(err),
			})
			continue
		}

		analysed = append(analysed, res.Report)
		if res.Attachment != nil {
			attachments = append(attachments, *res.Attachment)
		}
		result.Files = append(result.Files, QScanMasterResult{
			Outcome:  res.Report.Outcome.Value,
			JobID:    res.JobID,
			FileID:   res.FileID,
			Critical: res.Report.Critical,
			Warning:  res.Report.Warning,
			Logging:  res.Report.Logging,
		})
	}
	result.Analysed = len(analysed)
	result.Failed = len(failed)

	if len(failed) > 0 {
		// The uploader can do nothing about a QScan outage, so failures go to
		// the configured error addresses.
		recipients, err := qscanErrorRecipients(ctx)
		if err != nil {
			logger.Error("Failed to resolve the QScan error addresses", "error", err)
		} else {
			sendQScanReport(ctx, recipients, notifications.QScanBatchResult{Files: failed}, nil)
		}
	}

	if len(analysed) == 0 {
		return nil, fmt.Errorf("QC failed for all %d files: %w", len(in.Files), errors.Join(errs...))
	}

	report := notifications.QScanBatchResult{Files: analysed}
	result.Outcome = report.Outcome().Value

	if len(in.Recipients) == 0 {
		logger.Warn("No uploader to send the raw import QC report to, sending nothing", "outcome", result.Outcome, "files", len(analysed))
		return result, nil
	}
	sendQScanReport(ctx, in.Recipients, report, attachments)

	return result, nil
}

// qscanChildErrorMessage strips the child workflow wrapping so the mail says
// what QScan reported, not which Temporal execution failed.
func qscanChildErrorMessage(err error) string {
	var appErr *temporal.ApplicationError
	if errors.As(err, &appErr) {
		return appErr.Message()
	}
	return err.Error()
}
