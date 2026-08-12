package wfutils

import (
	"cmp"
	"slices"

	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
)

// SortedKeys returns the keys of m in ascending order.
//
// Workflow code must not range over a map: Go randomizes the iteration order,
// so a replay can visit the entries in a different order than the original run
// and schedule its activities in a different order. Ranging over SortedKeys
// instead gives the same order every time.
//
// Unlike GetMapKeysSafely this costs no history: the order is derived from the
// keys rather than recorded, so there is no SideEffect to replay. Prefer it
// wherever the key type is ordered.
//
//workflowcheck:ignore the map iteration below feeds a sort, so the result does not depend on it
func SortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// GetMapKeysSafely makes sure that the order of the keys returned are identical to other workflow executions.
func GetMapKeysSafely[K comparable, T any](ctx workflow.Context, m map[K]T) ([]K, error) {
	var keys []K
	err := workflow.SideEffect(ctx, func(ctx workflow.Context) any {
		return lo.Keys(m)
	}).Get(&keys)
	return keys, err
}
