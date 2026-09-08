package activities

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/qscan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

type qscanTestConfig struct{ baseURL string }

func (c qscanTestConfig) BaseURL() string  { return c.baseURL }
func (c qscanTestConfig) Username() string { return "u" }
func (c qscanTestConfig) Password() string { return "p" }

// fakeQScan is the smallest server that lets the ensure activities reconcile.
type fakeQScan struct {
	jobs  []qscan.Job
	files map[int64][]qscan.JobFile
	posts map[string]int
	// addedPaths records what was submitted, in submission order.
	addedPaths []string
	// analysed turns on the results ids, which QScan only hands out once the
	// analysis has produced results.
	analysed bool
	// reportedIDs records the ids the report endpoint was asked for.
	reportedIDs []string
	// reportStatus, when set, is what the report endpoint answers instead of a PDF.
	reportStatus int
}

// resultsID is 0 until the file has been analysed, as it is on the real server.
func (f *fakeQScan) resultsID(index int) qscan.Int {
	if !f.analysed {
		return 0
	}
	return qscan.Int(90 + index)
}

// listedFiles is what GET /jobs/1/files answers now.
func (f *fakeQScan) listedFiles() []qscan.JobFile {
	listed := make([]qscan.JobFile, len(f.files[1]))
	for i, file := range f.files[1] {
		file.ResultsID = f.resultsID(i)
		listed[i] = file
	}
	return listed
}

func newFakeQScan(t *testing.T) (*fakeQScan, *QScanActivities) {
	t.Helper()
	f := &fakeQScan{files: map[int64][]qscan.JobFile{}, posts: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api-1/qc/templates", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":7,"name":"BCCM - Masters","type":"custom"}]`))
	})
	mux.HandleFunc("/api-1/qc/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			f.posts["jobs"]++
			var req qscan.CreateJobRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			job := qscan.Job{ID: qscan.Int(len(f.jobs) + 1), Name: req.Name, Status: "running"}
			f.jobs = append(f.jobs, job)
			_ = json.NewEncoder(w).Encode(job)
			return
		}
		_ = json.NewEncoder(w).Encode(f.jobs)
	})
	mux.HandleFunc("/api-1/qc/jobs/1/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			f.posts["files"]++
			var req []qscan.AddFileRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			f.addedPaths = append(f.addedPaths, req[0].Path)
			// QScan stores and echoes the path in Windows form.
			file := qscan.JobFile{ID: qscan.Int(10 + len(f.files[1])), JobID: 1, ResultsID: f.resultsID(len(f.files[1])), Path: strings.ReplaceAll(req[0].Path, "/", `\`)}
			f.files[1] = append(f.files[1], file)
			_ = json.NewEncoder(w).Encode(file)
			return
		}
		if len(f.files[1]) == 0 {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"status":"Not Found","message":"Job ID has no files"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(f.listedFiles())
	})
	mux.HandleFunc("/api-1/qc/jobs/1/files/10/events", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"severity":"critical","events":[{"media_type":"video","message":"Freeze"}]}]`))
	})
	mux.HandleFunc("/api-1/qc/jobs/1/files/{resultsID}/report", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("resultsID")
		f.reportedIDs = append(f.reportedIDs, id)
		if f.reportStatus != 0 {
			w.WriteHeader(f.reportStatus)
			_, _ = w.Write([]byte(`{"status":"Not Found","message":"No report for this file"}`))
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4 fake"))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client, err := qscan.NewClient(qscanTestConfig{baseURL: server.URL})
	require.NoError(t, err)
	return f, &QScanActivities{Client: client, RepositoryID: 2, TemplateName: "BCCM - Masters"}
}

// newQScanEnv registers the activities under test. They log through the
// activity context, so they cannot be called with a plain context.
func newQScanEnv(t *testing.T, a *QScanActivities) *testsuite.TestActivityEnvironment {
	t.Helper()
	env := (&testsuite.WorkflowTestSuite{}).NewTestActivityEnvironment()
	env.RegisterActivity(a.QScanEnsureJob)
	env.RegisterActivity(a.QScanEnsureFile)
	env.RegisterActivity(a.QScanFetchResult)
	return env
}

func TestQScanEnsureJobAndFile_AreIdempotent(t *testing.T) {
	fake, a := newFakeQScan(t)
	env := newQScanEnv(t, a)

	master := paths.New(paths.IsilonDrive, "Production/masters/MASTER_01.mxf")
	in := QScanEnsureJobInput{VXID: "VX-1", Path: master}

	var first, second QScanJob
	val, err := env.ExecuteActivity(a.QScanEnsureJob, in)
	require.NoError(t, err)
	require.NoError(t, val.Get(&first))
	val, err = env.ExecuteActivity(a.QScanEnsureJob, in)
	require.NoError(t, err)
	require.NoError(t, val.Get(&second))

	assert.Equal(t, first, second)
	assert.Equal(t, "VX-1 MASTER_01.mxf", first.JobName)
	assert.Equal(t, 1, fake.posts["jobs"], "the second run must reuse the job")

	var f1, f2 QScanFile
	val, err = env.ExecuteActivity(a.QScanEnsureFile, QScanEnsureFileInput{JobID: first.JobID, Path: master})
	require.NoError(t, err)
	require.NoError(t, val.Get(&f1))
	val, err = env.ExecuteActivity(a.QScanEnsureFile, QScanEnsureFileInput{JobID: first.JobID, Path: master})
	require.NoError(t, err)
	require.NoError(t, val.Get(&f2))

	assert.Equal(t, f1, f2)
	assert.Equal(t, int64(10), f1.FileID)
	// A freshly queued file has no results yet, so the id it carries is 0 and
	// the fetch has to read it again once the analysis is done.
	assert.Zero(t, f1.ResultsID)
	assert.Equal(t, 1, fake.posts["files"], "the second run must find the queued file")

	// QScan concatenates the repository root and this path, so dropping the
	// leading separator would name the share "isilonProduction" and the
	// analysis would fail with "The network name cannot be found".
	assert.Equal(t, []string{"/Production/masters/MASTER_01.mxf"}, fake.addedPaths)
}

func TestQScanEnsureJob_RejectsNonIsilonPaths(t *testing.T) {
	_, a := newFakeQScan(t)

	_, err := a.QScanEnsureJob(context.Background(), QScanEnsureJobInput{VXID: "VX-1", Path: paths.New(paths.TempDrive, "x.mxf")})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "isilon")
}

// queueAndAnalyse puts the master in the job and lets the analysis finish, so
// the state matches what the fetch meets in production.
func queueAndAnalyse(t *testing.T, env *testsuite.TestActivityEnvironment, fake *fakeQScan, a *QScanActivities) {
	t.Helper()
	master := paths.New(paths.IsilonDrive, "Production/masters/MASTER_01.mxf")
	_, err := env.ExecuteActivity(a.QScanEnsureJob, QScanEnsureJobInput{VXID: "VX-1", Path: master})
	require.NoError(t, err)
	_, err = env.ExecuteActivity(a.QScanEnsureFile, QScanEnsureFileInput{JobID: 1, Path: master})
	require.NoError(t, err)
	fake.analysed = true
}

func testReportPath(t *testing.T) paths.Path {
	t.Helper()
	report := paths.New(paths.TestDrive, filepath.Join("generated", "qscan", "VX-1_qscan_report.pdf"))
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(report.Local())) })
	return report
}

func fetchQScanReport(t *testing.T, env *testsuite.TestActivityEnvironment, a *QScanActivities, in QScanFetchResultInput) QScanFetchResultOutput {
	t.Helper()
	val, err := env.ExecuteActivity(a.QScanFetchResult, in)
	require.NoError(t, err)
	var out QScanFetchResultOutput
	require.NoError(t, val.Get(&out))
	return out
}

func TestQScanFetchResult_WritesThePDFNextToTheWorkflow(t *testing.T) {
	fake, a := newFakeQScan(t)
	env := newQScanEnv(t, a)
	queueAndAnalyse(t, env, fake, a)
	report := testReportPath(t)

	out := fetchQScanReport(t, env, a, QScanFetchResultInput{JobID: 1, FileID: 10, ResultsID: 90, ReportPath: report})

	assert.True(t, out.ReportSaved)
	assert.Len(t, out.Events, 1)
	content, err := os.ReadFile(report.Local())
	require.NoError(t, err)
	assert.Equal(t, "%PDF-1.4 fake", string(content))
}

// The submitted results id is 0 because the file had not been analysed when it
// was queued; the report only exists under the id the job reports afterwards.
func TestQScanFetchResult_ReReadsTheResultsIDBeforeDownloading(t *testing.T) {
	fake, a := newFakeQScan(t)
	env := newQScanEnv(t, a)
	queueAndAnalyse(t, env, fake, a)
	report := testReportPath(t)

	out := fetchQScanReport(t, env, a, QScanFetchResultInput{JobID: 1, FileID: 10, ResultsID: 0, ReportPath: report})

	assert.True(t, out.ReportSaved)
	assert.Equal(t, int64(90), out.ResultsID)
	assert.Equal(t, []string{"90"}, fake.reportedIDs)
}

func TestQScanFetchResult_SaysNothingWasDownloadedWhenThereIsNoResultsID(t *testing.T) {
	fake, a := newFakeQScan(t)
	env := newQScanEnv(t, a)
	queueAndAnalyse(t, env, fake, a)
	fake.analysed = false
	report := testReportPath(t)

	out := fetchQScanReport(t, env, a, QScanFetchResultInput{JobID: 1, FileID: 10, ResultsID: 0, ReportPath: report})

	assert.False(t, out.ReportSaved)
	assert.Empty(t, fake.reportedIDs, "there is nothing to ask for")
	assert.Contains(t, out.ReportNote, "has not produced a report")
}

func TestQScanFetchResult_NamesTheReasonTheReportIsMissing(t *testing.T) {
	fake, a := newFakeQScan(t)
	env := newQScanEnv(t, a)
	queueAndAnalyse(t, env, fake, a)
	fake.reportStatus = http.StatusNotFound
	report := testReportPath(t)

	// A missing PDF must not lose the summary.
	out := fetchQScanReport(t, env, a, QScanFetchResultInput{JobID: 1, FileID: 10, ResultsID: 0, ReportPath: report})

	assert.False(t, out.ReportSaved)
	assert.Len(t, out.Events, 1)
	assert.Contains(t, out.ReportNote, "could not be fetched")
	assert.Contains(t, out.ReportNote, "404", "the note must say what actually failed")
}
