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

// viewPage is one page of the view-load response: the field definitions and a batch of
// task IDs.
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

// clickupServer routes the two calls ListTasks makes: a paged GET for the view, and a
// POST for the task values.
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

	client, err := NewClient(server.URL, "ws-1", "view-1", "token-1")
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

// A failed view load must not read as a view with no tasks: the shorts export would
// then quietly export nothing and report success.
func TestListTasks_ViewLoadFailureIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"err":"boom"}`))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL, "ws-1", "view-1", "token-1")
	require.NoError(t, err)

	tasks, err := client.ListTasks()

	require.Error(t, err)
	assert.Nil(t, tasks)
	assert.Contains(t, err.Error(), "500")
}

// The second call can fail on its own, after a successful view load.
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

	client, err := NewClient(server.URL, "ws-1", "view-1", "token-1")
	require.NoError(t, err)

	tasks, err := client.ListTasks()

	require.Error(t, err)
	assert.Nil(t, tasks)
}

// Paging continues until last_page.
func TestListTasks_PagesUntilLastPage(t *testing.T) {
	pages := []string{viewPage(false, "t1"), viewPage(true, "t2")}
	tasks := `{"tasks":[{"id":"t1","name":"First"},{"id":"t2","name":"Second"}]}`

	client, _ := clickupServer(t, pages, tasks)

	all, err := client.ListTasks()

	require.NoError(t, err)
	assert.Len(t, all, 2)
}

// The guard against a non-advancing page parameter: a view that keeps answering with
// the same task IDs must end the loop rather than page forever.
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

	client, err := NewClient(server.URL, "ws-1", "view-1", "token-1")
	require.NoError(t, err)

	_, err = client.ListTasks()

	require.NoError(t, err)
	assert.Equal(t, "token-1", token)
}

// The paths are relative to the client's base URL, so they are pinned against what
// the service actually receives.
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

	client, err := NewClient(server.URL, "ws-1", "view-1", "token-1")
	require.NoError(t, err)

	_, err = client.ListTasks()

	require.NoError(t, err)
	require.Len(t, paths, 2)
	assert.Equal(t, "GET /view/v1/ws-1/public/view/view-1", paths[0])
	assert.Equal(t, "POST /view/v1/ws-1/public/view/view-1/tasks", paths[1])
}

// Empty arguments fall back to the public "Shorts Export" view.
func TestNewClient_EmptyArgumentsUseTheDefaults(t *testing.T) {
	client, err := NewClient("", "", "", "")

	require.NoError(t, err)
	assert.Equal(t, defaultBaseURL, client.baseURL)
	assert.Equal(t, defaultWorkspaceID, client.workspaceID)
	assert.Equal(t, defaultViewID, client.viewID)
	assert.Equal(t, defaultToken, client.token)
	assert.True(t, strings.HasPrefix(client.baseURL, "https://"))
}

// A scheduled workflow calls this, so a ClickUp that accepts the connection and stops
// answering must fail rather than hold the activity.
func TestClient_HasATimeout(t *testing.T) {
	client, err := NewClient("", "", "", "")
	require.NoError(t, err)

	assert.Positive(t, client.client.GetClient().Timeout)
}
