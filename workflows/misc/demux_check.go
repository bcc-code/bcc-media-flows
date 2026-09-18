package miscworkflows

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	// demuxCheckMaxEmailMessages caps the messages per file in a mailed report.
	// The written report keeps everything the activity returned.
	demuxCheckMaxEmailMessages = 50

	// DemuxCheckReportSuffix is appended to a file's name for the report that
	// is written next to it when nobody is there to mail.
	DemuxCheckReportSuffix = ".demux-check.html"
)

type DemuxCheckInput struct {
	Files []paths.Path
	// Recipients get one mail with every verdict. Entries that are not
	// addresses are ignored, and with none left the report is written next to
	// each file instead.
	Recipients []string
}

type DemuxCheckFileResult struct {
	Path     paths.Path
	Outcome  string
	Errors   int
	Warnings int
	// ReportPath is where the HTML report was written, when it was.
	ReportPath *paths.Path
}

type DemuxCheckResult struct {
	// Outcome is the worst verdict among the files.
	Outcome string
	Files   []DemuxCheckFileResult
	// Mailed is true when the report went to the recipients rather than to disk.
	Mailed bool
}

// DemuxCheck reads every file through ffmpeg without decoding, so that a
// truncated or damaged file is known about before Mediabanken gets it, and
// reports what ffmpeg found. The report is mailed to the recipients when there
// are any, otherwise written as HTML next to each file.
//
// A broken file is a verdict, not an error. The workflow errors only when the
// report could not be delivered anywhere.
func DemuxCheck(ctx workflow.Context, in DemuxCheckInput) (*DemuxCheckResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting DemuxCheck workflow", "files", len(in.Files), "recipients", len(in.Recipients))

	if len(in.Files) == 0 {
		return nil, temporal.NewNonRetryableApplicationError("no files to check", "DemuxCheckNoFiles", nil)
	}

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	// Every file is checked at once; each is its own ffmpeg process on the
	// transcode queue, and the read is I/O bound.
	tasks := make([]wfutils.Task[*ffmpeg.DemuxCheckResult], len(in.Files))
	for i, file := range in.Files {
		tasks[i] = wfutils.Execute(ctx, activities.Video.DemuxCheck, activities.DemuxCheckParams{FilePath: file})
	}

	checkedAt := workflow.Now(ctx).Format("2006-01-02 15:04:05 MST")
	files := make([]notifications.DemuxCheckFile, len(in.Files))
	result := &DemuxCheckResult{}
	for i, task := range tasks {
		checked, err := task.Result(ctx)
		if err != nil {
			logger.Error("Demux check could not run", "path", in.Files[i].Linux(), "error", err)
			files[i] = notifications.DemuxCheckFile{
				Filename: in.Files[i].Base(),
				Path:     in.Files[i].Linux(),
				Outcome:  notifications.QCError,
				Error:    activityErrorMessage(err),
			}
		} else {
			files[i] = demuxCheckFileReport(in.Files[i], *checked)
		}

		result.Files = append(result.Files, DemuxCheckFileResult{
			Path:     in.Files[i],
			Outcome:  files[i].Outcome.Value,
			Errors:   files[i].Errors,
			Warnings: files[i].Warnings,
		})
	}
	result.Outcome = notifications.DemuxCheckReport{Files: files}.Outcome().Value

	recipients := emailRecipients(in.Recipients)
	if len(recipients) > 0 {
		report := notifications.DemuxCheckReport{Files: capDemuxCheckMessages(files, demuxCheckMaxEmailMessages), CheckedAt: checkedAt}
		wfutils.SendEmailTemplate(ctx, recipients, report)
		result.Mailed = true
		return result, nil
	}

	logger.Info("No recipient for the demux check report, writing it next to the files")
	var errs []error
	for i, file := range files {
		reportPath := in.Files[i].Dir().Append(in.Files[i].Base() + DemuxCheckReportSuffix)
		report := notifications.DemuxCheckReport{Files: []notifications.DemuxCheckFile{file}, CheckedAt: checkedAt}

		html, err := report.RenderHTML()
		if err != nil {
			errs = append(errs, fmt.Errorf("rendering report for %s: %w", in.Files[i].Base(), err))
			continue
		}
		if err := wfutils.WriteFile(ctx, reportPath, []byte(html)); err != nil {
			errs = append(errs, fmt.Errorf("writing report %s: %w", reportPath.Linux(), err))
			continue
		}
		result.Files[i].ReportPath = &reportPath
	}

	if len(errs) > 0 {
		return result, errors.Join(errs...)
	}
	return result, nil
}

// demuxCheckFileReport turns what ffmpeg found into the verdict for one file.
func demuxCheckFileReport(path paths.Path, checked ffmpeg.DemuxCheckResult) notifications.DemuxCheckFile {
	file := notifications.DemuxCheckFile{
		Filename:         path.Base(),
		Path:             path.Linux(),
		Errors:           checked.Errors,
		Warnings:         checked.Warnings,
		TotalMessages:    checked.TotalMessages,
		ProcessedSeconds: checked.ProcessedSeconds,
		TotalSeconds:     checked.TotalSeconds,
		ShortRead:        checked.ShortRead,
		ExitError:        checked.ExitError,
		Command:          checked.Command,
	}

	for _, m := range checked.Messages {
		file.Messages = append(file.Messages, notifications.DemuxCheckMessage{
			Level:     m.Level,
			Component: m.Component,
			Message:   m.Message,
		})
	}

	switch {
	case !checked.Passed():
		file.Outcome = notifications.QCFailed
	case checked.Warnings > 0:
		file.Outcome = notifications.QCPassedWithWarnings
	default:
		file.Outcome = notifications.QCPassed
	}

	return file
}

// capDemuxCheckMessages keeps the first limit messages of each file. Errors
// are kept ahead of warnings so a mail never shows only the harmless ones.
func capDemuxCheckMessages(files []notifications.DemuxCheckFile, limit int) []notifications.DemuxCheckFile {
	capped := make([]notifications.DemuxCheckFile, len(files))
	for i, file := range files {
		capped[i] = file
		if len(file.Messages) <= limit {
			continue
		}

		var selected []notifications.DemuxCheckMessage
		for _, wantErrors := range []bool{true, false} {
			for _, m := range file.Messages {
				if m.IsError() == wantErrors && len(selected) < limit {
					selected = append(selected, m)
				}
			}
		}
		capped[i].Messages = selected
	}
	return capped
}

// emailRecipients keeps the entries that look like addresses. Uploader fields
// are sometimes a name rather than a mail address, and those cannot be mailed.
func emailRecipients(recipients []string) []string {
	var out []string
	for _, r := range recipients {
		r = strings.TrimSpace(r)
		if strings.Contains(r, "@") {
			out = append(out, r)
		}
	}
	return out
}

// activityErrorMessage strips the Temporal wrapping so the report says what
// went wrong, not which activity execution failed.
func activityErrorMessage(err error) string {
	var appErr *temporal.ApplicationError
	if errors.As(err, &appErr) {
		return appErr.Message()
	}
	return err.Error()
}
