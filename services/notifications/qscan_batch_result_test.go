package notifications

import (
	"strings"
	"testing"
)

func sampleBatch() QScanBatchResult {
	return QScanBatchResult{Files: []QScanResult{
		{VXID: "VX-1", Filename: "CLIP_01.mxf", Outcome: QCPassed, Status: "analyzed", JobName: "VX-1 CLIP_01.mxf", QScanURL: "http://qscan"},
		{VXID: "VX-2", Filename: "CLIP_02.mxf", Outcome: QCFailed, Status: "analyzed", Critical: 1, Warning: 2, TotalEvents: 3,
			Events: []QScanEvent{{Severity: "critical", MediaType: "video", Message: "Freeze", TCIn: "00:00:01:00", TCOut: "00:00:03:00"}}},
	}}
}

func TestQScanBatchResult_Outcome(t *testing.T) {
	cases := []struct {
		name     string
		outcomes []QCOutcome
		want     QCOutcome
	}{
		{"empty", nil, QCPassed},
		{"all passed", []QCOutcome{QCPassed, QCPassed}, QCPassed},
		{"warnings", []QCOutcome{QCPassed, QCPassedWithWarnings}, QCPassedWithWarnings},
		{"failed beats warnings", []QCOutcome{QCPassedWithWarnings, QCFailed, QCPassed}, QCFailed},
		{"error beats failed", []QCOutcome{QCFailed, QCError}, QCError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var batch QScanBatchResult
			for _, o := range c.outcomes {
				batch.Files = append(batch.Files, QScanResult{Outcome: o})
			}
			if got := batch.Outcome(); got != c.want {
				t.Errorf("Outcome() = %q, want %q", got.Value, c.want.Value)
			}
		})
	}
}

func TestQScanBatchResult_Subject(t *testing.T) {
	if got, want := sampleBatch().Subject(), "QC FAILED: raw import, 2 files"; got != want {
		t.Errorf("Subject() = %q, want %q", got, want)
	}
	one := QScanBatchResult{Files: []QScanResult{{Outcome: QCError}}}
	if got, want := one.Subject(), "QC ERROR: raw import, 1 file"; got != want {
		t.Errorf("Subject() = %q, want %q", got, want)
	}
}

func TestQScanBatchResult_RenderMarkdown(t *testing.T) {
	batch := sampleBatch()
	batch.Files = append(batch.Files, QScanResult{VXID: "VX-3", Filename: "CLIP_03.mxf", Outcome: QCError, Error: "creating QScan job: boom"})

	got, err := batch.RenderMarkdown()
	if err != nil {
		t.Fatalf("RenderMarkdown() error: %v", err)
	}

	for _, want := range []string{
		"🟥 QC ERROR: raw import, 3 files",
		"1 passed, 1 failed, 1 could not be analysed",
		"VX-1",
		"VX-2",
		"VX-3",
		"CLIP\\_01.mxf",
		"Critical: 1, Warning: 2, Logging: 0",
		"Freeze",
		"```\ncreating QScan job: boom\n```",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderMarkdown() missing %q in:\n%s", want, got)
		}
	}
}

func TestQScanBatchResult_RenderHTML(t *testing.T) {
	batch := sampleBatch()
	got, err := batch.RenderHTML()
	if err != nil {
		t.Fatalf("RenderHTML() error: %v", err)
	}
	for _, want := range []string{
		"QC FAILED: raw import, 2 files",
		"CLIP_01.mxf",
		"CLIP_02.mxf",
		"QScan has analysed 2 files from your raw import. The full reports are attached.",
		"Events (1 of 3)",
		"Freeze",
		`<a href="http://qscan"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderHTML() missing %q", want)
		}
	}

	errored := QScanBatchResult{Files: []QScanResult{{VXID: "VX-9", Filename: "X.mxf", Outcome: QCError, Error: "boom"}}}
	got, err = errored.RenderHTML()
	if err != nil {
		t.Fatalf("RenderHTML() error: %v", err)
	}
	if !strings.Contains(got, "The QC run did not complete for this file.") || !strings.Contains(got, "boom") {
		t.Errorf("RenderHTML() for an errored batch lacks the error wording:\n%s", got)
	}
}

// The per-file block moved into a shared partial; the single-file mail must
// still render with it.
func TestQScanResult_RenderHTML_StillRenders(t *testing.T) {
	got, err := sampleBatch().Files[1].RenderHTML()
	if err != nil {
		t.Fatalf("RenderHTML() error: %v", err)
	}
	for _, want := range []string{"QScan has analysed the uploaded master.", "CLIP_02.mxf", "1 / 2 / 0", "Freeze"} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderHTML() missing %q", want)
		}
	}
}
