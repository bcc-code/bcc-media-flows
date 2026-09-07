package activities

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
			file := qscan.JobFile{ID: qscan.Int(10 + len(f.files[1])), JobID: 1, ResultsID: qscan.Int(90 + len(f.files[1])), Path: `\` + req[0].Path}
			f.files[1] = append(f.files[1], file)
			_ = json.NewEncoder(w).Encode(file)
			return
		}
		if len(f.files[1]) == 0 {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"status":"Not Found","message":"Job ID has no files"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(f.files[1])
	})
	mux.HandleFunc("/api-1/qc/jobs/1/files/10/events", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"severity":"critical","events":[{"media_type":"video","message":"Freeze"}]}]`))
	})
	mux.HandleFunc("/api-1/qc/jobs/1/files/90/report", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4 fake"))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client, err := qscan.NewClient(qscanTestConfig{baseURL: server.URL})
	require.NoError(t, err)
	return f, &QScanActivities{Client: client, RepositoryID: 2, TemplateName: "BCCM - Masters"}
}

func TestQScanEnsureJobAndFile_AreIdempotent(t *testing.T) {
	fake, a := newFakeQScan(t)
	env := (&testsuite.WorkflowTestSuite{}).NewTestActivityEnvironment()
	env.RegisterActivity(a.QScanEnsureJob)
	env.RegisterActivity(a.QScanEnsureFile)

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
	assert.Equal(t, int64(90), f1.ResultsID)
	assert.Equal(t, 1, fake.posts["files"], "the second run must find the queued file")
}

func TestQScanEnsureJob_RejectsNonIsilonPaths(t *testing.T) {
	_, a := newFakeQScan(t)

	_, err := a.QScanEnsureJob(context.Background(), QScanEnsureJobInput{VXID: "VX-1", Path: paths.New(paths.TempDrive, "x.mxf")})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "isilon")
}

func TestQScanFetchResult_WritesThePDFNextToTheWorkflow(t *testing.T) {
	_, a := newFakeQScan(t)
	report := paths.New(paths.TestDrive, filepath.Join("generated", "qscan", "VX-1_qscan_report.pdf"))
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(report.Local())) })

	out, err := a.QScanFetchResult(context.Background(), QScanFetchResultInput{JobID: 1, FileID: 10, ResultsID: 90, ReportPath: report})

	require.NoError(t, err)
	assert.True(t, out.ReportSaved)
	assert.Len(t, out.Events, 1)
	content, err := os.ReadFile(report.Local())
	require.NoError(t, err)
	assert.Equal(t, "%PDF-1.4 fake", string(content))
}
