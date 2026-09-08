package notifications

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/orsinium-labs/enum"
)

var (
	//go:embed templates/qscan_result.gohtml
	qscanResultTemplateString string
	qscanResultTemplate       = mustEmailTemplate("qscan_result", qscanResultTemplateString)
)

type QCOutcome enum.Member[string]

var (
	QCPassed             = QCOutcome{"PASSED"}
	QCPassedWithWarnings = QCOutcome{"PASSED WITH WARNINGS"}
	QCFailed             = QCOutcome{"FAILED"}
	QCError              = QCOutcome{"ERROR"}
	QCOutcomes           = enum.New(QCPassed, QCPassedWithWarnings, QCFailed, QCError)
)

type QScanEvent struct {
	Severity  string
	MediaType string
	Message   string
	TCIn      string
	TCOut     string
}

// QScanResult reports the outcome of a QScan analysis of one file.
type QScanResult struct {
	VXID     string
	Filename string
	Outcome  QCOutcome
	// Status is the raw QScan file status, StatusInfo its explanation.
	Status     string
	StatusInfo string
	Critical   int
	Warning    int
	Logging    int
	// Events is a capped selection; TotalEvents is how many QScan found.
	Events      []QScanEvent
	TotalEvents int
	JobName     string
	QScanURL    string
	// Error is set for QCError.
	Error string
	// ReportNote explains a missing attachment.
	ReportNote string
}

func (t QScanResult) Passed() bool {
	return t.Outcome == QCPassed || t.Outcome == QCPassedWithWarnings
}

func (t QScanResult) Subject() string {
	return fmt.Sprintf("QC %s: %s %s", t.Outcome.Value, t.VXID, t.Filename)
}

func (t QScanResult) RenderHTML() (string, error) {
	return renderHtmlTemplate(qscanResultTemplate, t)
}

func (t QScanResult) RenderMarkdown() (string, error) {
	var b strings.Builder

	icon := "🟥"
	if t.Passed() {
		icon = "✅"
	}
	fmt.Fprintf(&b, "%s QC %s: `%s` %s\n", icon, t.Outcome.Value, escapeCode(t.VXID), escapeMarkdown(t.Filename))
	if t.Error != "" {
		fmt.Fprintf(&b, "```\n%s\n```\n", escapeCodeBlock(t.Error))
	}
	if t.Status != "" {
		fmt.Fprintf(&b, "Status: %s", escapeMarkdown(oneLine(t.Status)))
		if t.StatusInfo != "" {
			fmt.Fprintf(&b, " (%s)", escapeMarkdown(oneLine(t.StatusInfo)))
		}
		b.WriteString("\n")
	}
	if t.Outcome != QCError {
		fmt.Fprintf(&b, "Critical: %d, Warning: %d, Logging: %d\n", t.Critical, t.Warning, t.Logging)
	}
	if len(t.Events) > 0 {
		fmt.Fprintf(&b, "\nEvents (%d of %d):\n", len(t.Events), t.TotalEvents)
		for _, e := range t.Events {
			fmt.Fprintf(&b, "- \\[%s] %s %s", escapeMarkdown(e.Severity), escapeMarkdown(e.MediaType), escapeMarkdown(oneLine(e.Message)))
			if e.TCIn != "" {
				fmt.Fprintf(&b, " @ %s", escapeMarkdown(e.TCIn))
				if e.TCOut != "" && e.TCOut != e.TCIn {
					fmt.Fprintf(&b, " - %s", escapeMarkdown(e.TCOut))
				}
			}
			b.WriteString("\n")
		}
	}
	if t.ReportNote != "" {
		fmt.Fprintf(&b, "\n%s\n", escapeMarkdown(t.ReportNote))
	}
	if t.JobName != "" {
		fmt.Fprintf(&b, "\nQScan job: %s", escapeMarkdown(t.JobName))
		if t.QScanURL != "" {
			// The configured QScan base URL, left unescaped so Telegram still
			// turns it into a link.
			fmt.Fprintf(&b, " (%s)", t.QScanURL)
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}
