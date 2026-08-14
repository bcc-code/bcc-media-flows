package activities

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The trigger tells BMM to pick up the exported bmm.json. 201 Created is a plausible
// answer for an endpoint that creates an event, and it means the import was triggered
// — reporting it as a failure retries an import that already started.
func TestTriggerBMMImport_AnySuccessStatusIsASuccess(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))
			t.Cleanup(server.Close)

			env := activityEnv(t)
			ua := UtilActivities{}
			env.RegisterActivity(ua.TriggerBMMImport)

			_, err := env.ExecuteActivity(ua.TriggerBMMImport, TriggerBMMImportInput{
				BaseURL:      server.URL,
				IngestFolder: "/mnt/ingest/AABC",
			})

			assert.NoError(t, err)
		})
	}
}

func TestTriggerBMMImport_ServerErrorIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"no such path"}`))
	}))
	t.Cleanup(server.Close)

	env := activityEnv(t)
	ua := UtilActivities{}
	env.RegisterActivity(ua.TriggerBMMImport)

	_, err := env.ExecuteActivity(ua.TriggerBMMImport, TriggerBMMImportInput{
		BaseURL:      server.URL,
		IngestFolder: "/mnt/ingest/AABC",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// The path is a query parameter, and it has to arrive as the path itself rather than
// as something the escaping mangled.
func TestTriggerBMMImport_SendsThePathToTheSidecar(t *testing.T) {
	var gotPath, gotEndpoint, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEndpoint, gotMethod = r.URL.Path, r.Method
		gotPath = r.URL.Query().Get("path")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	env := activityEnv(t)
	ua := UtilActivities{}
	env.RegisterActivity(ua.TriggerBMMImport)

	_, err := env.ExecuteActivity(ua.TriggerBMMImport, TriggerBMMImportInput{
		BaseURL:      server.URL,
		IngestFolder: "/mnt/ingest/AABC 2024/S01 E01",
	})

	require.NoError(t, err)
	assert.Equal(t, "/events/mediabanken-export/", gotEndpoint)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/mnt/ingest/AABC 2024/S01 E01/bmm.json", gotPath,
		"a folder name with a space must arrive unmangled")
}
