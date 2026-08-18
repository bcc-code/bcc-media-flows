package clickup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testConfig struct {
	baseURL     string
	workspaceID string
	viewID      string
	token       string
}

func (c testConfig) FrontdoorBaseURL() string { return c.baseURL }
func (c testConfig) WorkspaceID() string      { return c.workspaceID }
func (c testConfig) ShortsViewID() string     { return c.viewID }
func (c testConfig) ShortsViewToken() string  { return c.token }

func viewPage(lastPage bool, taskIDs ...string) string {
	ids, _ := json.Marshal(taskIDs)
	last := "false"
	if lastPage {
		last = "true"
	}
	return `{
		"custom_fields":[{"id":"f1","name":"Status","type":"drop_down",
			"type_config":{"options":[{"id":"o1","name":"Ready"}]}}],
		"last_page":` + last + `,
		"list":{"divisions":[{"groups":[{"task_ids":` + string(ids) + `}]}]}}`
}

func clickupServer(t *testing.T, pages []string, tasksBody string) (*Client, *int) {
	t.Helper()

	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost {
			posts++
			_, _ = w.Write([]byte(tasksBody))
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page >= len(pages) {
			page = len(pages) - 1
		}
		_, _ = w.Write([]byte(pages[page]))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(testConfig{baseURL: server.URL, workspaceID: "ws-1", viewID: "view-1", token: "token-1"})
	require.NoError(t, err)

	return client, &posts
}

func TestListTasks_SuccessAssemblesTasksFromBothCalls(t *testing.T) {
	tasks := `{"tasks":[{"id":"t1","name":"First","customFieldValues":[
		{"field_id":"f1","value":"o1"}]}]}`

	client, posts := clickupServer(t, []string{viewPage(true, "t1")}, tasks)

	all, err := client.ListTasks()

	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "t1", all[0].ID)
	assert.Equal(t, "First", all[0].Name)
	require.Len(t, all[0].CustomFields, 1)
	assert.Equal(t, "Status", all[0].CustomFields[0].Name)
	assert.Equal(t, "drop_down", all[0].CustomFields[0].Type)
	assert.Equal(t, 1, *posts)
}

func TestListTasks_ViewLoadFailureIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"err":"boom"}`))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(testConfig{baseURL: server.URL, workspaceID: "ws-1", viewID: "view-1", token: "token-1"})
	require.NoError(t, err)

	tasks, err := client.ListTasks()

	require.Error(t, err)
	assert.Nil(t, tasks)
	assert.Contains(t, err.Error(), "500")
}

func TestListTasks_TaskFetchFailureIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("<html>502</html>"))
			return
		}
		_, _ = w.Write([]byte(viewPage(true, "t1")))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(testConfig{baseURL: server.URL, workspaceID: "ws-1", viewID: "view-1", token: "token-1"})
	require.NoError(t, err)

	tasks, err := client.ListTasks()

	require.Error(t, err)
	assert.Nil(t, tasks)
}

func TestListTasks_PagesUntilLastPage(t *testing.T) {
	pages := []string{viewPage(false, "t1"), viewPage(true, "t2")}
	tasks := `{"tasks":[{"id":"t1","name":"First"},{"id":"t2","name":"Second"}]}`

	client, _ := clickupServer(t, pages, tasks)

	all, err := client.ListTasks()

	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestListTasks_StopsWhenAPageAddsNothingNew(t *testing.T) {
	repeated := viewPage(false, "t1")
	tasks := `{"tasks":[{"id":"t1","name":"First"}]}`

	client, _ := clickupServer(t, []string{repeated}, tasks)

	all, err := client.ListTasks()

	require.NoError(t, err)
	assert.Len(t, all, 1)
}

func TestClient_SendsTheShareToken(t *testing.T) {
	var token string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			token = r.URL.Query().Get("token")
		}
		_, _ = w.Write([]byte(viewPage(true)))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(testConfig{baseURL: server.URL, workspaceID: "ws-1", viewID: "view-1", token: "token-1"})
	require.NoError(t, err)

	_, err = client.ListTasks()

	require.NoError(t, err)
	assert.Equal(t, "token-1", token)
}

func TestClient_RequestsTheViewPaths(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"tasks":[]}`))
			return
		}
		_, _ = w.Write([]byte(viewPage(true, "t1")))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(testConfig{baseURL: server.URL, workspaceID: "ws-1", viewID: "view-1", token: "token-1"})
	require.NoError(t, err)

	_, err = client.ListTasks()

	require.NoError(t, err)
	require.Len(t, paths, 2)
	assert.Equal(t, "GET /view/v1/ws-1/public/view/view-1", paths[0])
	assert.Equal(t, "POST /view/v1/ws-1/public/view/view-1/tasks", paths[1])
}

func TestNewClient_EmptyArgumentsUseTheDefaults(t *testing.T) {
	client, err := NewClient(testConfig{baseURL: "", workspaceID: "", viewID: "", token: ""})

	require.NoError(t, err)
	assert.Equal(t, defaultBaseURL, client.baseURL)
	assert.Equal(t, defaultWorkspaceID, client.workspaceID)
	assert.Equal(t, defaultViewID, client.viewID)
	assert.Equal(t, defaultToken, client.token)
	assert.True(t, strings.HasPrefix(client.baseURL, "https://"))
}

func TestClient_HasATimeout(t *testing.T) {
	client, err := NewClient(testConfig{baseURL: "", workspaceID: "", viewID: "", token: ""})
	require.NoError(t, err)

	assert.Positive(t, client.client.GetClient().Timeout)
}
