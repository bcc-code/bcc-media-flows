package utils_test

import (
	"os"
	"testing"

	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/stretchr/testify/assert"
)

func TestIsDirEmpty(t *testing.T) {
	emptyDir, err := os.MkdirTemp("", "emptydir")
	assert.NoError(t, err)

	empty, err := utils.IsDirEmpty(emptyDir)
	assert.NoError(t, err)
	assert.True(t, empty)

	empty, err = utils.IsDirEmpty("/")
	assert.NoError(t, err)
	assert.False(t, empty)

	empty, err = utils.IsDirEmpty("/this/path/does/not/exist")
	assert.Error(t, err)
	assert.False(t, empty)
}
