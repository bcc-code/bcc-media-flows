package notifications

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleDemuxCheckReport() DemuxCheckReport {
	return DemuxCheckReport{
		CheckedAt: "2026-09-17 10:00:00 UTC",
		Files: []DemuxCheckFile{
			{
				Filename:         "CLIP_01.mxf",
				Path:             "/mnt/isilon/Production/raw/CLIP_01.mxf",
				Outcome:          QCPassed,
				ProcessedSeconds: 3,
				TotalSeconds:     3,
				Command:          "ffmpeg -i CLIP_01.mxf -c copy -f null -",
			},
			{
				Filename:         "CLIP_02.mxf",
				Path:             "/mnt/isilon/Production/raw/CLIP_02.mxf",
				Outcome:          QCFailed,
				Errors:           2,
				Warnings:         1,
				ProcessedSeconds: 1.84,
				TotalSeconds:     3,
				ShortRead:        true,
				TotalMessages:    3,
				Messages: []DemuxCheckMessage{
					{Level: "warning", Component: "in#0/mxf", Message: "Packet corrupt (stream = 1, dts = NOPTS)."},
					{Level: "error", Component: "h264", Message: "Invalid NAL unit size (351865195 > 58)."},
					{Level: "error", Component: "demux check", Message: "Reading stopped at 00:00:01.84 of the 00:00:03.00 the file reports"},
				},
			},
		},
	}
}

func TestDemuxCheckReport_Outcome(t *testing.T) {
	report := sampleDemuxCheckReport()
	assert.Equal(t, QCFailed, report.Outcome())
	assert.False(t, report.Passed())

	passed, failed, errored := report.Counts()
	assert.Equal(t, 1, passed)
	assert.Equal(t, 1, failed)
	assert.Equal(t, 0, errored)

	assert.Equal(t, "File check FAILED: 2 files", report.Subject())

	single := DemuxCheckReport{Files: report.Files[:1]}
	assert.Equal(t, QCPassed, single.Outcome())
	assert.Equal(t, "File check PASSED: CLIP_01.mxf", single.Subject())

	assert.Equal(t, QCPassed, DemuxCheckReport{}.Outcome())
}

func TestDemuxCheckReport_RenderHTML(t *testing.T) {
	html, err := sampleDemuxCheckReport().RenderHTML()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(strings.TrimSpace(html), "<!DOCTYPE html>"), "the report is a complete page so it can be written to disk")
	for _, want := range []string{
		"File check FAILED: 2 files",
		"CLIP_01.mxf",
		"CLIP_02.mxf",
		"/mnt/isilon/Production/raw/CLIP_02.mxf",
		"00:00:01.84 of 00:00:03.00",
		"ends before its header says",
		"Invalid NAL unit size (351865195 &gt; 58).",
		"without a single warning",
		"Checked 2026-09-17 10:00:00 UTC",
	} {
		assert.Contains(t, html, want)
	}
	assert.NotContains(t, html, "first 3 of 3", "no note when every message is shown")
}

func TestDemuxCheckReport_RenderHTMLErrorAndCappedMessages(t *testing.T) {
	report := DemuxCheckReport{Files: []DemuxCheckFile{
		{Filename: "A.wav", Path: "/a/A.wav", Outcome: QCError, Error: "ffmpeg: executable file not found"},
		{Filename: "B.wav", Path: "/b/B.wav", Outcome: QCFailed, Errors: 300, TotalMessages: 300, ExitError: "ffmpeg exited with 1: Invalid data found when processing input",
			Messages: []DemuxCheckMessage{{Level: "error", Message: "x"}}},
	}}

	html, err := report.RenderHTML()
	require.NoError(t, err)
	assert.Contains(t, html, "The check did not run")
	assert.Contains(t, html, "executable file not found")
	assert.Contains(t, html, "ffmpeg could not read the file")
	assert.Contains(t, html, "first 1 of 300")
	assert.Equal(t, "File check ERROR: 2 files", report.Subject())
}

func TestDemuxCheckReport_RenderMarkdown(t *testing.T) {
	md, err := sampleDemuxCheckReport().RenderMarkdown()
	require.NoError(t, err)

	for _, want := range []string{
		"🟥 File check FAILED: 2 files",
		"1 passed, 1 failed, 0 could not be checked",
		"PASSED: `/mnt/isilon/Production/raw/CLIP_01.mxf`",
		"Read 00:00:01.84 of 00:00:03.00",
		"Errors: 2, Warnings: 1",
		"Messages (3 of 3):",
		"- \\[error] h264: Invalid NAL unit size (351865195 > 58).",
	} {
		assert.Contains(t, md, want)
	}

	single, err := DemuxCheckReport{Files: sampleDemuxCheckReport().Files[:1]}.RenderMarkdown()
	require.NoError(t, err)
	assert.Contains(t, single, `✅ File check PASSED: CLIP\_01.mxf`, "filenames outside code spans are escaped for Telegram's legacy Markdown")
}

func TestFormatSeconds(t *testing.T) {
	assert.Equal(t, "00:00:00.00", FormatSeconds(0))
	assert.Equal(t, "00:01:30.50", FormatSeconds(90.5))
	assert.Equal(t, "02:00:00.00", FormatSeconds(7200))
}
