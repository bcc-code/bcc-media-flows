package bootstrap

import (
	"github.com/bcc-code/bcc-media-flows/environment"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentity(t *testing.T) {
	t.Setenv("IDENTITY", "")
	environment.Load()
	assert.Equal(t, "worker", Identity())

	t.Setenv("IDENTITY", "transcode-01")
	environment.Load()
	assert.Equal(t, "transcode-01", Identity())
}

func TestTemporalClient_RequiresAHost(t *testing.T) {
	t.Setenv("TEMPORAL_HOST_PORT", "")
	environment.Load()

	_, err := TemporalClient()

	require.Error(t, err, "dialing an empty host fails later and less clearly")
	assert.Contains(t, err.Error(), "TEMPORAL_HOST_PORT")
}
