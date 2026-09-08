package notifications

import (
	"strings"
	"testing"
)

// legacyMarkdownBalanced approximates Telegram's legacy Markdown parser: it
// walks the text, honouring backslash escapes, and reports the delimiters left
// open. Telegram answers an unclosed entity with "Bad Request: can't parse
// entities", refusing the whole message.
func legacyMarkdownUnclosed(md string) []string {
	var open []string
	for i := 0; i < len(md); i++ {
		switch md[i] {
		case '\\':
			i++ // The escaped character is literal.
		case '`':
			if strings.HasPrefix(md[i:], "```") {
				i += 2
				open = toggle(open, "```")
				continue
			}
			open = toggle(open, "`")
		case '_', '*':
			if len(open) > 0 && (open[len(open)-1] == "```" || open[len(open)-1] == "`") {
				continue // Inside a code entity nothing else is markup.
			}
			open = toggle(open, string(md[i]))
		case '[':
			if len(open) > 0 && (open[len(open)-1] == "```" || open[len(open)-1] == "`") {
				continue
			}
			if !strings.Contains(md[i:], "](") {
				open = append(open, "[")
			}
		}
	}
	return open
}

func toggle(open []string, delim string) []string {
	if len(open) > 0 && open[len(open)-1] == delim {
		return open[:len(open)-1]
	}
	return append(open, delim)
}

// The message that Telegram refused in production: three underscores in the
// filename, so the third opened an italic entity that swallowed the opening
// code fence and left the closing one dangling.
func qscanFileError() QScanResult {
	return QScanResult{
		VXID:       "VX-519484",
		Filename:   "TP01_2_2_TESTING.mov",
		Outcome:    QCError,
		Status:     "file_error",
		StatusInfo: "The network name cannot be found.\r\n",
		Error:      "QScan did not analyse the file: status file_error The network name cannot be found.\r\n",
		JobName:    "VX-519484 TP01_2_2_TESTING.mov",
		QScanURL:   "http://10.12.140.18:8080",
	}
}

func TestQScanResult_RenderMarkdownClosesEveryEntity(t *testing.T) {
	got, err := qscanFileError().RenderMarkdown()
	if err != nil {
		t.Fatalf("RenderMarkdown() error: %v", err)
	}
	if unclosed := legacyMarkdownUnclosed(got); len(unclosed) != 0 {
		t.Fatalf("unclosed entities %v in:\n%s", unclosed, got)
	}
	for _, want := range []string{
		`TP01\_2\_2\_TESTING.mov`,
		"Status: file\\_error (The network name cannot be found.)",
		"QScan job: VX-519484 TP01\\_2\\_2\\_TESTING.mov (http://10.12.140.18:8080)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered message is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\r") {
		t.Errorf("carriage return survived into the message:\n%q", got)
	}
}

// A failed QC lists events, and the "[SEVERITY]" prefix is itself a legacy
// Markdown link opener.
func TestQScanResult_RenderMarkdownEscapesEvents(t *testing.T) {
	report := qscanFileError()
	report.Outcome = QCFailed
	report.Error = ""
	report.Events = []QScanEvent{
		{Severity: "CRITICAL", MediaType: "Video", Message: "Black frames [start_end] detected", TCIn: "00:00:01:00"},
	}
	report.TotalEvents = 1

	got, err := report.RenderMarkdown()
	if err != nil {
		t.Fatalf("RenderMarkdown() error: %v", err)
	}
	if unclosed := legacyMarkdownUnclosed(got); len(unclosed) != 0 {
		t.Fatalf("unclosed entities %v in:\n%s", unclosed, got)
	}
	if want := `- \[CRITICAL] Video Black frames \[start\_end] detected @ 00:00:01:00`; !strings.Contains(got, want) {
		t.Errorf("rendered event is not %q:\n%s", want, got)
	}
}

func TestRenderMarkdownEscapesEveryTemplate(t *testing.T) {
	// A name that opens one of each legacy delimiter, plus a backtick that
	// cannot be escaped inside a code entity.
	hostile := "a_b*c`d[e"
	templates := map[string]Template{
		"QScanResult": QScanResult{VXID: hostile, Filename: hostile, Outcome: QCError, Error: hostile, Status: hostile, JobName: hostile},
		"ImportTriggered": ImportTriggered{
			Filename: hostile, UploadedBy: hostile, UploadedAt: hostile,
			Details: []DetailRow{{Label: hostile, Value: hostile}},
		},
		"ImportCompleted": ImportCompleted{JobID: hostile, Files: []File{{Name: hostile}}},
		"ImportFailed":    ImportFailed{JobID: hostile, Error: hostile, Files: []File{{Name: hostile}}},
	}

	for name, template := range templates {
		t.Run(name, func(t *testing.T) {
			got, err := template.RenderMarkdown()
			if err != nil {
				t.Fatalf("RenderMarkdown() error: %v", err)
			}
			if unclosed := legacyMarkdownUnclosed(got); len(unclosed) != 0 {
				t.Fatalf("unclosed entities %v in:\n%s", unclosed, got)
			}
		})
	}
}

func TestEscapeHelpers(t *testing.T) {
	if got, want := escapeMarkdown("a_b*c`d[e]f"), "a\\_b\\*c\\`d\\[e]f"; got != want {
		t.Errorf("escapeMarkdown() = %q, want %q", got, want)
	}
	if got, want := escapeCode("say `hi`"), "say 'hi'"; got != want {
		t.Errorf("escapeCode() = %q, want %q", got, want)
	}
	if got, want := escapeCodeBlock("\r\nfirst\r\nsecond\r\n"), "first\nsecond"; got != want {
		t.Errorf("escapeCodeBlock() = %q, want %q", got, want)
	}
	if got, want := oneLine(" status \r\n info\t"), "status info"; got != want {
		t.Errorf("oneLine() = %q, want %q", got, want)
	}
}
