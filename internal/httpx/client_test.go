package httpx

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serving returns a client pointed at a server that answers everything the same way.
func serving(t *testing.T, status int, contentType, body string) *resty.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return New(Config{Service: "testservice", BaseURL: server.URL})
}

// The whole point of the package: a status the caller never looks at still fails.
func TestNew_NonSuccessStatusIsAnError(t *testing.T) {
	client := serving(t, http.StatusInternalServerError, "application/json", `{"error":"boom"}`)

	result := &struct {
		Name string `json:"name"`
	}{}
	_, err := client.R().SetResult(result).Get("/thing")

	require.Error(t, err, "resty leaves err nil for a 500; the hook must not")
	assert.Empty(t, result.Name)
}

// A body resty will not unmarshal — an HTML page from a proxy — is the case a
// JSON-shaped check misses.
func TestNew_HTMLErrorBodyIsAnError(t *testing.T) {
	client := serving(t, http.StatusBadGateway, "text/html", "<html>502 Bad Gateway</html>")

	_, err := client.R().Get("/thing")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
}

func TestNew_ErrorNamesServiceRequestAndStatus(t *testing.T) {
	client := serving(t, http.StatusForbidden, "text/plain", "nope")

	_, err := client.R().Get("/items/VX-1")

	require.Error(t, err)
	message := err.Error()
	assert.Contains(t, message, "testservice")
	assert.Contains(t, message, http.MethodGet)
	assert.Contains(t, message, "/items/VX-1")
	assert.Contains(t, message, "403")
	assert.Contains(t, message, "nope")
}

// The status travels with the error, so a caller can branch on it without parsing text.
func TestNew_ErrorCarriesHTTPCode(t *testing.T) {
	client := serving(t, http.StatusNotFound, "text/plain", "gone")

	_, err := client.R().Get("/thing")

	require.Error(t, err)
	assert.Equal(t, http.StatusNotFound, merry.HTTPCode(err))
}

func TestNew_SuccessPassesThroughAndUnmarshals(t *testing.T) {
	client := serving(t, http.StatusOK, "application/json", `{"name":"ok"}`)

	result := &struct {
		Name string `json:"name"`
	}{}
	resp, err := client.R().SetResult(result).Get("/thing")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
	assert.Equal(t, "ok", result.Name)
}

// 201 and 204 are successes. A check that compares against 200 exactly makes a
// correct Created read as a failure.
func TestNew_CreatedAndNoContentAreNotErrors(t *testing.T) {
	for _, status := range []int{http.StatusCreated, http.StatusNoContent} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := serving(t, status, "application/json", "")

			_, err := client.R().Post("/thing")

			assert.NoError(t, err)
		})
	}
}

func TestTolerating_LeavesTheListedStatusToTheCaller(t *testing.T) {
	client := serving(t, http.StatusNotFound, "application/json", `{"detail":"no such item"}`)

	resp, err := Tolerating(client.R(), http.StatusNotFound).Get("/items/VX-1")

	require.NoError(t, err, "a tolerated status is an answer, not a failure")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode())
}

// Tolerating one status must not tolerate the rest of the failures on that request.
func TestTolerating_OtherStatusesStillFail(t *testing.T) {
	client := serving(t, http.StatusInternalServerError, "text/plain", "boom")

	_, err := Tolerating(client.R(), http.StatusNotFound).Get("/items/VX-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// Toleration is per request, not per client.
func TestTolerating_DoesNotLeakToOtherRequests(t *testing.T) {
	client := serving(t, http.StatusNotFound, "text/plain", "gone")

	_, err := Tolerating(client.R(), http.StatusNotFound).Get("/probe")
	require.NoError(t, err)

	_, err = client.R().Get("/fetch")
	require.Error(t, err, "the next request must not inherit the toleration")
}

func TestDescribeError_OverridesTheDefaultMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(server.Close)

	sentinel := fmt.Errorf("this service says so")
	client := New(Config{
		Service: "custom",
		BaseURL: server.URL,
		DescribeError: func(resp *resty.Response) error {
			return fmt.Errorf("%d: %w", resp.StatusCode(), sentinel)
		},
	})

	_, err := client.R().Get("/thing")

	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
}

// The error names the request, and subtrans authenticates with ?key=, so redaction is
// what keeps that key out of the Temporal workflow history.
func TestNew_ErrorDoesNotLeakACredentialFromTheQueryString(t *testing.T) {
	client := serving(t, http.StatusInternalServerError, "text/plain", "boom")

	_, err := client.R().
		SetQueryParams(map[string]string{"key": "s3cr3t-api-key", "incLanguages": "true"}).
		Get("/api/external/story/files/name")

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "s3cr3t-api-key")
	assert.Contains(t, err.Error(), "redacted")
	assert.Contains(t, err.Error(), "incLanguages=true", "non-secret parameters stay readable")
}

func TestRedactURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wants   []string
		unwants []string
	}{
		{
			name:    "key",
			url:     "http://subtrans/api/story?key=secret&incLanguages=true",
			wants:   []string{"key=redacted", "incLanguages=true"},
			unwants: []string{"secret"},
		},
		{
			name:    "capitalised name",
			url:     "http://svc/thing?Token=secret",
			wants:   []string{"redacted"},
			unwants: []string{"secret"},
		},
		{
			name:    "several at once",
			url:     "http://svc/thing?api_key=one&access_token=two",
			unwants: []string{"one", "two"},
		},
		{
			name:  "nothing to redact is returned unchanged",
			url:   "http://svc/items/VX-1?content=metadata&terse=true",
			wants: []string{"http://svc/items/VX-1?content=metadata&terse=true"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := RedactURL(test.url)
			for _, want := range test.wants {
				assert.Contains(t, got, want)
			}
			for _, unwanted := range test.unwants {
				assert.NotContains(t, got, unwanted)
			}
		})
	}
}

// A URL that will not parse must not be echoed: whatever made it unparseable could be
// the credential itself.
func TestRedactURL_UnparseableIsNotEchoed(t *testing.T) {
	got := RedactURL("://not a url\x7f?key=secret")

	assert.NotContains(t, got, "secret")
}

// Retries exist for connection failures. An error the response hook produced must not
// be retried: it means the server answered, and answering again would create a second
// shape or job for a POST.
func TestNew_ErrorStatusIsNotRetried(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := New(Config{Service: "retrying", BaseURL: server.URL, RetryCount: 5})

	_, err := client.R().Post("/thing")

	require.Error(t, err)
	assert.Equal(t, 1, calls, "a 500 answered by the server is not a reason to POST again")
}

func TestTruncateBody_BoundsWhatReachesWorkflowHistory(t *testing.T) {
	short := strings.Repeat("a", maxErrorBodyLen)
	assert.Equal(t, short, TruncateBody([]byte(short)), "a body at the limit is kept whole")

	long := strings.Repeat("a", maxErrorBodyLen*3)
	truncated := TruncateBody([]byte(long))
	assert.Less(t, len(truncated), len(long))
	assert.Contains(t, truncated, "truncated")
}

// A hung server must fail the attempt rather than hold the worker until the activity's
// schedule-to-close budget runs out.
func TestNew_TimeoutBoundsTheAttempt(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})

	client := New(Config{Service: "slow", BaseURL: server.URL, Timeout: 50 * time.Millisecond})

	started := time.Now()
	_, err := client.R().Get("/thing")

	require.Error(t, err)
	assert.Less(t, time.Since(started), 5*time.Second)
}

func TestNew_HeadersAndBasicAuthAreSentOnEveryRequest(t *testing.T) {
	var gotToken, gotUser, gotPassword string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("Auth-Token")
		gotUser, gotPassword, _ = r.BasicAuth()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := New(Config{
		Service:   "authed",
		BaseURL:   server.URL,
		Headers:   map[string]string{"Auth-Token": "secret"},
		BasicAuth: &BasicAuth{Username: "user", Password: "pass"},
	})

	_, err := client.R().Get("/thing")

	require.NoError(t, err)
	assert.Equal(t, "secret", gotToken)
	assert.Equal(t, "user", gotUser)
	assert.Equal(t, "pass", gotPassword)
}

// A trailing slash on the configured base URL must not produce "//path".
func TestNew_BaseURLTrailingSlashIsTrimmed(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := New(Config{Service: "based", BaseURL: server.URL + "/"})

	_, err := client.R().Get("/items/VX-1")

	require.NoError(t, err)
	assert.Equal(t, "/items/VX-1", path)
}
