package cantemo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskInProgress(t *testing.T) {
	for state, want := range map[string]bool{
		"PENDING": true,
		"STARTED": true,
		"SUCCESS": false,
		"FAILURE": false,
		"REVOKED": false,
		"":        false,
	} {
		assert.Equal(t, want, taskInProgress(state), state)
	}
}
