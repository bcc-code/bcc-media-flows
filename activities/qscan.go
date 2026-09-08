package activities

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/qscan"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// QScan exposes activities for the Quales QScan automated QC server.
var QScan = &QScanActivities{}

type QScanActivities struct {
	Client *qscan.Client
	// RepositoryID is the QScan repository rooted at the isilon share.
	RepositoryID int64
	TemplateName string
}

func (a *QScanActivities) ready() error {
	if a.Client == nil {
		return temporal.NewNonRetryableApplicationError("qscan client not configured", "qscan_not_configured", nil)
	}
	return nil
}

type QScanEnsureJobInput struct {
	VXID string
	Path paths.Path
}

type QScanJob struct {
	JobID   int64
	JobName string
	JobURL  string
}

// QScanJobName is deterministic so a retried submission finds the job it
// already created instead of making another.
func QScanJobName(vxid string, path paths.Path) string {
	return strings.TrimSpace(vxid + " " + path.Base())
}

// QScanEnsureJob returns the job for this master, creating it only if no job
// with its name exists yet.
func (a *QScanActivities) QScanEnsureJob(ctx context.Context, in QScanEnsureJobInput) (*QScanJob, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if in.Path.Drive != paths.IsilonDrive {
		return nil, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("qscan only reaches the isilon repository, not drive %q", in.Path.Drive.Value),
			"qscan_unreachable_path", nil)
	}

	logger := activity.GetLogger(ctx)
	jobName := QScanJobName(in.VXID, in.Path)

	job, err := a.Client.FindJobByName(ctx, jobName)
	if err != nil {
		return nil, err
	}

	if job == nil {
		template, err := a.Client.FindTemplate(ctx, a.TemplateName)
		if errors.Is(err, qscan.ErrTemplateNotFound) {
			return nil, temporal.NewNonRetryableApplicationError(err.Error(), "qscan_template_not_found", err)
		}
		if err != nil {
			return nil, err
		}

		job, err = a.Client.CreateJob(ctx, qscan.CreateJobRequest{
			Name:                jobName,
			Description:         "Automatic QC of uploaded master " + in.VXID,
			TemplateID:          int64(template.ID),
			TemplateName:        template.Name,
			ConcurrentAnalysis:  1,
			AnalysisPerformance: 10,
			ReportSeverity:      1,
		})
		if err != nil {
			return nil, err
		}
	} else {
		logger.Info("Reusing existing QScan job", "jobID", job.ID, "name", jobName)
	}

	if job.Status != "running" {
		if err := a.Client.ResumeJob(ctx, int64(job.ID)); err != nil {
			logger.Warn("Could not resume QScan job, files may stay queued", "jobID", job.ID, "status", job.Status, "error", err)
		}
	}

	return &QScanJob{
		JobID:   int64(job.ID),
		JobName: jobName,
		JobURL:  a.Client.BaseURL,
	}, nil
}

type QScanEnsureFileInput struct {
	JobID int64
	Path  paths.Path
}

type QScanFile struct {
	FileID    int64
	ResultsID int64
}

// QScanEnsureFile queues the file in the job unless it is already there.
func (a *QScanActivities) QScanEnsureFile(ctx context.Context, in QScanEnsureFileInput) (*QScanFile, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}

	// The path is relative to the QScan repository and must keep its leading
	// separator: QScan appends it to the repository root verbatim, so without
	// one the first component is glued onto the share name
	// (\\server\isilonProduction\...) and Windows answers
	// "The network name cannot be found".
	repoPath := "/" + strings.TrimLeft(in.Path.Path, "/")

	existing, err := a.Client.ListJobFiles(ctx, in.JobID)
	if err != nil {
		return nil, err
	}
	for _, f := range existing {
		if sameRepoPath(f.Path, repoPath) {
			activity.GetLogger(ctx).Info("File already queued in QScan job", "jobID", in.JobID, "fileID", f.ID)
			return &QScanFile{FileID: int64(f.ID), ResultsID: int64(f.ResultsID)}, nil
		}
	}

	file, err := a.Client.AddFile(ctx, in.JobID, qscan.AddFileRequest{
		RepositoryID: a.RepositoryID,
		Path:         repoPath,
	})
	if err != nil {
		return nil, err
	}
	return &QScanFile{FileID: int64(file.ID), ResultsID: int64(file.ResultsID)}, nil
}

// sameRepoPath compares paths as the Windows host may echo them back: with
// backslashes and a leading separator.
func sameRepoPath(a, b string) bool {
	norm := func(p string) string {
		return strings.TrimLeft(strings.ReplaceAll(p, `\`, "/"), "/")
	}
	return strings.EqualFold(norm(a), norm(b))
}

type QScanFileStatusInput struct {
	JobID  int64
	FileID int64
}

func (a *QScanActivities) QScanFileStatus(ctx context.Context, in QScanFileStatusInput) (*qscan.FileStatus, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.Client.FileStatus(ctx, in.JobID, in.FileID)
}

type QScanFetchResultInput struct {
	JobID     int64
	FileID    int64
	ResultsID int64
	// ReportPath is where the PDF is written; it stays out of the workflow
	// history and is read again by the email activity.
	ReportPath paths.Path
}

type QScanFetchResultOutput struct {
	Events []qscan.Event
	// ReportSaved is false when no PDF was written; ReportNote then says why.
	ReportSaved bool
	ReportNote  string
}

// QScanFetchResult collects the events and saves the PDF report. A missing PDF
// is reported in the output rather than failing: the summary is still worth
// sending.
func (a *QScanActivities) QScanFetchResult(ctx context.Context, in QScanFetchResultInput) (*QScanFetchResultOutput, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}

	events, err := a.Client.FileEvents(ctx, in.JobID, in.FileID)
	if err != nil {
		return nil, err
	}
	out := &QScanFetchResultOutput{Events: events}

	pdf, err := a.Client.DownloadReport(ctx, in.JobID, in.ResultsID, qscan.ReportPDF)
	if err == nil {
		local := in.ReportPath.Local()
		if err = os.MkdirAll(filepath.Dir(local), 0o755); err == nil {
			err = os.WriteFile(local, pdf, 0o644)
		}
	}
	if err != nil {
		activity.GetLogger(ctx).Warn("Could not save QScan PDF report", "jobID", in.JobID, "resultsID", in.ResultsID, "error", err)
		out.ReportNote = "The PDF report could not be fetched; open it in QScan."
		return out, nil
	}

	out.ReportSaved = true
	return out, nil
}
