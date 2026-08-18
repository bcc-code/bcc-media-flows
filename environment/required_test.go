package environment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWarnMissing_ReportsEveryUnsetVariableAtOnce(t *testing.T) {
	t.Setenv("SET_ONE", "value")
	t.Setenv("UNSET_ONE", "")
	t.Setenv("UNSET_TWO", "")

	missing := WarnMissing([]string{"SET_ONE", "UNSET_ONE", "UNSET_TWO"})

	assert.Equal(t, []string{"UNSET_ONE", "UNSET_TWO"}, missing,
		"one report, not one discovery per variable")
}

func TestWarnMissing_SaysNothingWhenEverythingIsSet(t *testing.T) {
	t.Setenv("SET_ONE", "value")

	assert.Empty(t, WarnMissing([]string{"SET_ONE"}))
}

// The lists are what a reader consults to configure a deployment, so a duplicate or a
// stray empty entry is worth catching here.
func TestRequiredLists_AreCleanAndNonEmpty(t *testing.T) {
	lists := map[string][]string{
		"worker":      RequiredByWorker,
		"trigger_ui":  RequiredByTriggerUI,
		"httpin":      RequiredByHTTPIn,
		"bmm-trigger": RequiredByBMMTrigger,
	}

	for name, list := range lists {
		assert.NotEmpty(t, list, "%s requires nothing?", name)

		seen := map[string]bool{}
		for _, v := range list {
			assert.NotEmpty(t, v, "%s has an empty entry", name)
			assert.False(t, seen[v], "%s lists %s twice", name, v)
			seen[v] = true
		}

		assert.Contains(t, list, "TEMPORAL_HOST_PORT", "%s talks to Temporal", name)
	}
}
