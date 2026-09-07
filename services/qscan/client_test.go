package qscan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testConfig struct{ baseURL string }

func (c testConfig) BaseURL() string  { return c.baseURL }
func (c testConfig) Username() string { return "api" }
func (c testConfig) Password() string { return "secret" }

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient(testConfig{baseURL: server.URL + "/"})
	require.NoError(t, err)
	return client
}

func TestNewClient_RequiresABaseURL(t *testing.T) {
	client, err := NewClient(testConfig{})

	require.Error(t, err)
	assert.Nil(t, client)
}

func TestFindTemplate_MatchesByNameAndSendsBasicAuth(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "api", user)
		assert.Equal(t, "secret", pass)
		assert.Equal(t, "/api-1/qc/templates", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":"3","name":"Other","type":"default"},{"id":7,"name":"BCCM - Masters ","type":"custom"}]`))
	})

	tpl, err := client.FindTemplate(context.Background(), "bccm - masters")

	require.NoError(t, err)
	assert.Equal(t, Int(7), tpl.ID)
	assert.Equal(t, "BCCM - Masters ", tpl.Name)
}

func TestFindTemplate_MissingIsErrTemplateNotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	})

	_, err := client.FindTemplate(context.Background(), "BCCM - Masters")

	require.ErrorIs(t, err, ErrTemplateNotFound)
}

func TestCreateJob_SendsTemplateIDAndName(t *testing.T) {
	var body map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api-1/qc/jobs", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, _ = w.Write([]byte(`{"id":42,"status":"running","name":"VX-1 a.mxf"}`))
	})

	job, err := client.CreateJob(context.Background(), CreateJobRequest{
		Name: "VX-1 a.mxf", TemplateID: 7, TemplateName: "BCCM - Masters",
		ConcurrentAnalysis: 1, AnalysisPerformance: 10, ReportSeverity: 1,
	})

	require.NoError(t, err)
	assert.Equal(t, Int(42), job.ID)
	assert.Equal(t, float64(7), body["template_id"])
	assert.Equal(t, "BCCM - Masters", body["template_name"])
	assert.Equal(t, float64(1), body["concurrent_analysis"])
}

func TestAddFile_SendsAListAndAcceptsObjectOrList(t *testing.T) {
	for name, response := range map[string]string{
		"object": `{"id":"11","job_id":42,"results_id":"99","path":"Production/masters/a.mxf"}`,
		"list":   `[{"id":11,"job_id":42,"results_id":99,"path":"Production/masters/a.mxf"}]`,
	} {
		t.Run(name, func(t *testing.T) {
			var body []map[string]any
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api-1/qc/jobs/42/files", r.URL.Path)
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				_, _ = w.Write([]byte(response))
			})

			file, err := client.AddFile(context.Background(), 42, AddFileRequest{RepositoryID: 2, Path: "Production/masters/a.mxf"})

			require.NoError(t, err)
			require.Len(t, body, 1)
			assert.Equal(t, float64(2), body[0]["repository_id"])
			assert.Equal(t, "Production/masters/a.mxf", body[0]["path"])
			assert.Equal(t, Int(11), file.ID)
			assert.Equal(t, Int(99), file.ResultsID)
		})
	}
}

func TestFileStatus_DecodesCountsAndStatus(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api-1/qc/jobs/42/files/11/status", r.URL.Path)
		_, _ = w.Write([]byte(`{"status":"analyzed","progress":"100","critical_total":2,"warning_total":"5","logging_total":0}`))
	})

	status, err := client.FileStatus(context.Background(), 42, 11)

	require.NoError(t, err)
	assert.Equal(t, StatusAnalyzed, status.Status)
	assert.True(t, status.Status.IsTerminal())
	assert.True(t, status.Status.IsSuccess())
	assert.Equal(t, Int(2), status.CriticalTotal)
	assert.Equal(t, Int(5), status.WarningTotal)
}

func TestStatus_UnknownValueIsNotTerminal(t *testing.T) {
	var status FileStatus
	require.NoError(t, json.Unmarshal([]byte(`{"status":"something_new"}`), &status))

	assert.False(t, status.Status.IsTerminal())
	assert.Equal(t, "something_new", status.Status.String())

	out, err := json.Marshal(status)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"status":"something_new"`)
}

func TestFileEvents_FlattensSeverityGroups(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"severity":"critical","events":[{"media_type":"video","message":"Freeze","tc_in":"00:00:01:00","tc_out":"00:00:03:00"}]},
			{"severity":"warning","events":[{"media_type":"audio","message":"Mute","tc_in":"00:00:05:00","tc_out":"00:00:06:00","channel":"1"}]}
		]`))
	})

	events, err := client.FileEvents(context.Background(), 42, 11)

	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, "critical", events[0].Severity)
	assert.Equal(t, "Freeze", events[0].Message)
	assert.Equal(t, "warning", events[1].Severity)
	assert.Equal(t, Int(1), events[1].Channel)
}

func TestFileEvents_NotFoundMeansNoEvents(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status":"Not Found","message":"Events not found for file ID"}`))
	})

	events, err := client.FileEvents(context.Background(), 42, 11)

	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestDownloadReport_ReturnsBytes(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api-1/qc/jobs/42/files/99/report", r.URL.Path)
		assert.Equal(t, "pdf", r.URL.Query().Get("type"))
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4 fake"))
	})

	pdf, err := client.DownloadReport(context.Background(), 42, 99, ReportPDF)

	require.NoError(t, err)
	assert.Equal(t, "%PDF-1.4 fake", string(pdf))
}

func TestErrors_UseTheAPIEnvelope(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":"Bad Request","message":"Repository ID doesn't exist"}`))
	})

	_, err := client.AddFile(context.Background(), 42, AddFileRequest{RepositoryID: 9, Path: "x"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Repository ID doesn't exist")
	assert.Contains(t, err.Error(), "400")
}

func TestRetries_OnlyReads(t *testing.T) {
	calls := map[string]int{}
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls[r.Method]++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"status":"Internal Error","message":"boom"}`))
	})

	_, err := client.CreateJob(context.Background(), CreateJobRequest{Name: "x"})
	require.Error(t, err)
	assert.Equal(t, 1, calls[http.MethodPost], "a failed POST must not be replayed")

	_, err = client.ListJobs(context.Background())
	require.Error(t, err)
	assert.Equal(t, 4, calls[http.MethodGet], "a failed GET is retried three times")
}

func TestFindJobByName_PrefersTheNewestExactMatch(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api-1/qc/jobs", r.URL.Path)
		_, _ = w.Write([]byte(`[
			{"id":1,"name":"VX-1 a.mxf","status":"running"},
			{"id":"3","name":"VX-1 a.mxf","status":"paused"},
			{"id":2,"name":"VX-1 a.mxf (old)","status":"running"}
		]`))
	})

	job, err := client.FindJobByName(context.Background(), "VX-1 a.mxf")
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, Int(3), job.ID)

	job, err = client.FindJobByName(context.Background(), "VX-2 b.mxf")
	require.NoError(t, err)
	assert.Nil(t, job)
}

func TestListJobFiles_AcceptsObjectListAndNotFound(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
		want   int
	}{
		"object":   {http.StatusOK, `{"id":11,"job_id":42,"results_id":99,"path":"a.mxf"}`, 1},
		"list":     {http.StatusOK, `[{"id":11,"job_id":42,"results_id":99,"path":"a.mxf"},{"id":12,"job_id":42,"results_id":100,"path":"b.mxf"}]`, 2},
		"no files": {http.StatusNotFound, `{"status":"Not Found","message":"Job ID has no files"}`, 0},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api-1/qc/jobs/42/files", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})

			files, err := client.ListJobFiles(context.Background(), 42)

			require.NoError(t, err)
			assert.Len(t, files, tc.want)
		})
	}
}
