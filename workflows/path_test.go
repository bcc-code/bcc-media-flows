package workflows

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_CreateUniquePath(t *testing.T) {
	path := createUniquePath("")
	if len(path) != 10 {
		t.Fail()
	}
	assert.Equal(t, 48, len(path), "Invalid length")
}
