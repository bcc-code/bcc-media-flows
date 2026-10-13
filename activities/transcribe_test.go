package activities

import (
	"errors"
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
)

func TestTranscribe_WithoutAClientIsNotRetried(t *testing.T) {
	env := activityEnv(t)
	ua := UtilActivities{}
	env.RegisterActivity(ua.Transcribe)

	_, err := env.ExecuteActivity(ua.Transcribe, TranscribeParams{
		File:            paths.MustParse("/mnt/temp/in.wav"),
		DestinationPath: paths.MustParse("/mnt/temp/out"),
		Language:        "no",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "transcriber client not configured")

	var appErr *temporal.ApplicationError
	require.True(t, errors.As(err, &appErr))
	assert.True(t, appErr.NonRetryable())
}
