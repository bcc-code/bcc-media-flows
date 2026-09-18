package notifications

import (
	_ "embed"
	"fmt"
	"strings"
	"time"
)

var (
	//go:embed templates/demux_check.gohtml
	demuxCheckTemplateString string
	demuxCheckTemplate       = mustEmailTemplate("demux_check", demuxCheckTemplateString)
)

// DemuxCheckMessage is one line ffmpeg logged while reading a file.
type DemuxCheckMessage struct {
	// Level is warning, error, fatal or panic.
	Level     string
	Component string
	Message   string
}

// IsError is true for anything above warning level.
func (m DemuxCheckMessage) IsError() bool {
	return m.Level != "warning"
}

// DemuxCheckFile is the verdict for one file.
type DemuxCheckFile struct {
	Filename string
	Path     string
	Outcome  QCOutcome

	Errors   int
	Warnings int

	// Messages is a capped selection in log order; TotalMessages is how many
	// ffmpeg logged.
	Messages      []DemuxCheckMessage
	TotalMessages int

	// ProcessedSeconds is how far ffmpeg read; TotalSeconds is the duration the
	// file declares. ShortRead is set when the first falls well short of the
	// second.
	ProcessedSeconds float64
	TotalSeconds     float64
	ShortRead        bool

	// ExitError is set when ffmpeg could not read the file at all.
	ExitError string
	// Error is set for QCError, when the check itself did not run.
	Error string

	Command string
}

func (f DemuxCheckFile) Passed() bool {
	return f.Outcome == QCPassed || f.Outcome == QCPassedWithWarnings
}

// Processed and Total format the durations as hh:mm:ss.
func (f DemuxCheckFile) Processed() string { return FormatSeconds(f.ProcessedSeconds) }
func (f DemuxCheckFile) Total() string     { return FormatSeconds(f.TotalSeconds) }

// HasMoreMessages is true when the selection does not show everything.
func (f DemuxCheckFile) HasMoreMessages() bool {
	return f.TotalMessages > len(f.Messages)
}

// DemuxCheckReport reports the ffmpeg demux check of the files of one upload.
// It renders the same whether it is mailed or written next to the file.
type DemuxCheckReport struct {
	Files []DemuxCheckFile
	// CheckedAt is when the check ran, already formatted.
	CheckedAt string
}

// Outcome is the worst verdict among the files; PASSED when there are none.
func (t DemuxCheckReport) Outcome() QCOutcome {
	worst := QCPassed
	for _, f := range t.Files {
		if qcOutcomeRank[f.Outcome] > qcOutcomeRank[worst] {
			worst = f.Outcome
		}
	}
	return worst
}

func (t DemuxCheckReport) Passed() bool {
	outcome := t.Outcome()
	return outcome == QCPassed || outcome == QCPassedWithWarnings
}

// Counts splits the files into passed (with or without warnings), failed and
// not checked.
func (t DemuxCheckReport) Counts() (passed, failed, errored int) {
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

// FileCount reads "1 file" or "N files".
func (t DemuxCheckReport) FileCount() string {
	if len(t.Files) == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", len(t.Files))
}

func (t DemuxCheckReport) Subject() string {
	if len(t.Files) == 1 {
		return fmt.Sprintf("File check %s: %s", t.Outcome().Value, t.Files[0].Filename)
	}
	return fmt.Sprintf("File check %s: %s", t.Outcome().Value, t.FileCount())
}

func (t DemuxCheckReport) RenderHTML() (string, error) {
	return renderHtmlTemplate(demuxCheckTemplate, t)
}

func (t DemuxCheckReport) RenderMarkdown() (string, error) {
	var b strings.Builder

	icon := "🟥"
	if t.Passed() {
		icon = "✅"
	}
	fmt.Fprintf(&b, "%s %s\n", icon, escapeMarkdown(t.Subject()))
	if len(t.Files) > 1 {
		passed, failed, errored := t.Counts()
		fmt.Fprintf(&b, "%d passed, %d failed, %d could not be checked\n", passed, failed, errored)
	}

	for _, f := range t.Files {
		b.WriteString("\n")
		fmt.Fprintf(&b, "%s: `%s`\n", escapeMarkdown(f.Outcome.Value), escapeCode(f.Path))
		if f.Error != "" {
			fmt.Fprintf(&b, "```\n%s\n```\n", escapeCodeBlock(f.Error))
			continue
		}
		if f.ExitError != "" {
			fmt.Fprintf(&b, "ffmpeg could not read the file: %s\n", escapeMarkdown(oneLine(f.ExitError)))
		}
		if f.TotalSeconds > 0 {
			fmt.Fprintf(&b, "Read %s of %s\n", escapeMarkdown(f.Processed()), escapeMarkdown(f.Total()))
		}
		fmt.Fprintf(&b, "Errors: %d, Warnings: %d\n", f.Errors, f.Warnings)
		if len(f.Messages) > 0 {
			fmt.Fprintf(&b, "Messages (%d of %d):\n", len(f.Messages), f.TotalMessages)
			for _, m := range f.Messages {
				fmt.Fprintf(&b, "- \\[%s]", escapeMarkdown(m.Level))
				if m.Component != "" {
					fmt.Fprintf(&b, " %s:", escapeMarkdown(m.Component))
				}
				fmt.Fprintf(&b, " %s\n", escapeMarkdown(oneLine(m.Message)))
			}
		}
	}

	if t.CheckedAt != "" {
		fmt.Fprintf(&b, "\nChecked %s\n", escapeMarkdown(t.CheckedAt))
	}

	return b.String(), nil
}

// FormatSeconds renders a duration as hh:mm:ss.cc.
func FormatSeconds(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second)).Round(10 * time.Millisecond)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := d.Seconds() - float64(h*3600+m*60)
	return fmt.Sprintf("%02d:%02d:%05.2f", h, m, s)
}
