package wfutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortedKeysIsOrdered(t *testing.T) {
	got := SortedKeys(map[string]int{"nor": 1, "eng": 2, "deu": 3, "abk": 4})
	assert.Equal(t, []string{"abk", "deu", "eng", "nor"}, got)
}

// The point of the helper is that the result cannot depend on Go's randomized
// map iteration order, so run it enough times that an unsorted implementation
// would be seen returning something else.
func TestSortedKeysIsStableAcrossIterations(t *testing.T) {
	m := map[string]int{}
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		m[k] = 0
	}

	want := SortedKeys(m)
	for i := 0; i < 100; i++ {
		assert.Equal(t, want, SortedKeys(m))
	}
}

func TestSortedKeysOnEmptyAndNilMaps(t *testing.T) {
	assert.Empty(t, SortedKeys(map[string]int{}))
	assert.Empty(t, SortedKeys(map[string]int(nil)))
}
