package notifications

import (
	_ "embed"
	"fmt"
	"strings"
)

var (
	//go:embed templates/qscan_batch_result.gohtml
	qscanBatchResultTemplateString string
	qscanBatchResultTemplate       = mustEmailTemplate("qscan_batch_result", qscanPartials+qscanBatchResultTemplateString)
)

// qcOutcomeRank orders outcomes from best to worst so a batch can report the
// worst verdict among its files.
var qcOutcomeRank = map[QCOutcome]int{
	QCPassed:             0,
	QCPassedWithWarnings: 1,
	QCFailed:             2,
	QCError:              3,
}

// QScanBatchResult reports the QC of every video file in one upload in a
// single mail.
type QScanBatchResult struct {
	Files []QScanResult
}

// Outcome is the worst verdict among the files; PASSED when there are none.
func (t QScanBatchResult) Outcome() QCOutcome {
	worst := QCPassed
	for _, f := range t.Files {
		if qcOutcomeRank[f.Outcome] > qcOutcomeRank[worst] {
			worst = f.Outcome
		}
	}
	return worst
}

func (t QScanBatchResult) Passed() bool {
	outcome := t.Outcome()
	return outcome == QCPassed || outcome == QCPassedWithWarnings
}

// Counts splits the files into passed (with or without warnings), failed and
// not analysed.
func (t QScanBatchResult) Counts() (passed, failed, errored int) {
	for _, f := range t.Files {
		switch f.Outcome {
		case QCError:
			errored++
		case QCFailed:
			failed++
		default:
			passed++
		}
	}
	return passed, failed, errored
}

// AllErrors is true when no file was analysed at all.
func (t QScanBatchResult) AllErrors() bool {
	_, _, errored := t.Counts()
	return len(t.Files) > 0 && errored == len(t.Files)
}

// HasAttachments is true when at least one analysed file has a PDF report.
func (t QScanBatchResult) HasAttachments() bool {
	for _, f := range t.Files {
		if f.Outcome != QCError && f.ReportNote == "" {
			return true
		}
	}
	return false
}

// FileCount reads "1 file" or "N files".
func (t QScanBatchResult) FileCount() string {
	if len(t.Files) == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", len(t.Files))
}

func (t QScanBatchResult) Subject() string {
	return fmt.Sprintf("QC %s: raw import, %s", t.Outcome().Value, t.FileCount())
}

func (t QScanBatchResult) RenderHTML() (string, error) {
	return renderHtmlTemplate(qscanBatchResultTemplate, t)
}

func (t QScanBatchResult) RenderMarkdown() (string, error) {
	var b strings.Builder

	icon := "🟥"
	if t.Passed() {
		icon = "✅"
	}
	fmt.Fprintf(&b, "%s QC %s: raw import, %s\n", icon, t.Outcome().Value, t.FileCount())

	passed, failed, errored := t.Counts()
	fmt.Fprintf(&b, "%d passed, %d failed, %d could not be analysed\n", passed, failed, errored)

	for _, f := range t.Files {
		md, err := f.RenderMarkdown()
		if err != nil {
			return "", err
		}
		b.WriteString("\n")
		b.WriteString(md)
	}

	return b.String(), nil
}
