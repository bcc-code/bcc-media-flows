package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func authResult(t *testing.T, configuredKey, sentKey string) (bool, int, string) {
	t.Helper()
	t.Setenv(massiveWebhookKeyVar, configuredKey)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/webhook/massive", nil)
	if sentKey != "" {
		ctx.Request.Header.Set("api-key", sentKey)
	}

	ok := massiveWebhookAuthorized(ctx)
	return ok, recorder.Code, recorder.Body.String()
}

// An unset key used to mean "no key required", which left an endpoint that starts a
// copy workflow open to anyone who could reach it.
func TestMassiveWebhook_UnsetKeyRefuses(t *testing.T) {
	ok, code, body := authResult(t, "", "")

	require.False(t, ok)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Contains(t, body, massiveWebhookKeyVar,
		"the refusal names the variable, so the cause is not a guess")
}

func TestMassiveWebhook_UnsetKeyRefusesEvenWithAKeySent(t *testing.T) {
	ok, code, _ := authResult(t, "", "some-key")

	require.False(t, ok, "an attacker-supplied key must not satisfy an unset one")
	assert.Equal(t, http.StatusServiceUnavailable, code)
}

func TestMassiveWebhook_WrongKeyIsUnauthorized(t *testing.T) {
	ok, code, _ := authResult(t, "correct-key", "wrong-key")

	require.False(t, ok)
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestMassiveWebhook_MissingHeaderIsUnauthorized(t *testing.T) {
	ok, code, _ := authResult(t, "correct-key", "")

	require.False(t, ok)
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestMassiveWebhook_CorrectKeyPasses(t *testing.T) {
	ok, code, _ := authResult(t, "correct-key", "correct-key")

	require.True(t, ok)
	assert.Equal(t, http.StatusOK, code, "nothing is written when the request is allowed")
}

// A prefix of the key must not pass, which a length-insensitive compare could allow.
func TestMassiveWebhook_PrefixDoesNotPass(t *testing.T) {
	ok, _, _ := authResult(t, "correct-key", "correct")

	require.False(t, ok)
}
