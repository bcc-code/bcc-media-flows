package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
)

// stubClient satisfies client.Client by embedding the interface, so only the one
// method the handler calls needs implementing. Anything else would nil-panic, which
// is the desired signal if a handler starts reaching for more.
type stubClient struct {
	client.Client

	run client.WorkflowRun
	err error
}

func (c stubClient) ExecuteWorkflow(context.Context, client.StartWorkflowOptions, any, ...any) (client.WorkflowRun, error) {
	return c.run, c.err
}

type stubRun struct {
	client.WorkflowRun
}

func (stubRun) GetID() string    { return "wf-1" }
func (stubRun) GetRunID() string { return "run-1" }

// withClient points the package-level temporalClient at a stub for one test.
func withClient(t *testing.T, c client.Client) {
	t.Helper()

	previous := temporalClient
	temporalClient = c
	t.Cleanup(func() { temporalClient = previous })
}

func triggerRequest(t *testing.T, job, query string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/trigger/:job", triggerHandler)

	url := "/trigger/" + job
	if query != "" {
		url += "?" + query
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, url, nil))
	return rec
}

// The switch had no default, so an unknown or renamed job left res and err both nil
// and fell through to the 200 at the end of the handler. Callers read a typo as a
// successfully started workflow.
func Test_TriggerHandler_UnknownJob_IsNotReportedAsSuccess(t *testing.T) {
	// No stub needed: the default case is reached before the client is used.
	rec := triggerRequest(t, "ThisJobDoesNotExist", "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "unknown job")
	assert.Contains(t, rec.Body.String(), "ThisJobDoesNotExist")
}

// A trigger that cannot start its workflow must answer 500. The NormalizeAudio case
// declares its own `target, err` with `:=`, so a failure to start is invisible to any
// check placed after the switch — which answers 200 with a null body.
func Test_TriggerHandler_WorkflowStartFailure_Returns500(t *testing.T) {
	withClient(t, stubClient{err: errors.New("temporal is unreachable")})

	rec := triggerRequest(t, "NormalizeAudio", "targetLUFS=-23&file=/mnt/isilon/x.wav")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "temporal is unreachable")
}

// The same case for a job that never had the shadowing problem, to show the check
// after the switch works generally.
func Test_TriggerHandler_WorkflowStartFailure_Returns500_OtherJob(t *testing.T) {
	withClient(t, stubClient{err: errors.New("temporal is unreachable")})

	rec := triggerRequest(t, "UpdateAssetRelations", "vxID=VX-1")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "temporal is unreachable")
}

// Parameter validation still rejects before anything is started.
func Test_TriggerHandler_UnparseableTargetLUFS_Returns400(t *testing.T) {
	withClient(t, stubClient{run: stubRun{}})

	rec := triggerRequest(t, "NormalizeAudio", "targetLUFS=not-a-number")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// A started workflow is still a 200, so the guards are not just failing everything.
func Test_TriggerHandler_Success_Returns200(t *testing.T) {
	withClient(t, stubClient{run: stubRun{}})

	rec := triggerRequest(t, "NormalizeAudio", "targetLUFS=-23&file=/mnt/isilon/x.wav")

	assert.Equal(t, http.StatusOK, rec.Code)
}

// A case that neither starts a workflow nor writes its own response must not be
// reported as success either.
func Test_TriggerHandler_NilRunWithoutError_Returns500(t *testing.T) {
	withClient(t, stubClient{run: nil, err: nil})

	rec := triggerRequest(t, "NormalizeAudio", "targetLUFS=-23&file=/mnt/isilon/x.wav")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "did not start a workflow")
}

// Guards against the old behaviour specifically: a 200 whose body is the JSON null
// that a nil client.WorkflowRun marshals to.
func Test_TriggerHandler_NeverReturns200WithNullBody(t *testing.T) {
	for _, tc := range []struct {
		name  string
		job   string
		query string
		stub  stubClient
	}{
		{"unknown job", "NoSuchJob", "", stubClient{}},
		{"start failure", "NormalizeAudio", "targetLUFS=-23", stubClient{err: errors.New("nope")}},
		{"nil run", "NormalizeAudio", "targetLUFS=-23", stubClient{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withClient(t, tc.stub)

			rec := triggerRequest(t, tc.job, tc.query)

			if rec.Code == http.StatusOK {
				var body any
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				require.NotNil(t, body, "a 200 must never carry a null body")
			}
			assert.NotEqual(t, http.StatusOK, rec.Code, "this request did not start a workflow")
		})
	}
}
