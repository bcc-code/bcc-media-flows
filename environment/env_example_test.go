package environment

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The example files drifted because nothing checked them: cmd/worker documented
// TEMPORAL_ADDRESS while the code read TEMPORAL_HOST_PORT, and trigger_ui had two
// files disagreeing with each other.
func TestEnvExamplesDocumentWhatEachEntrypointNeeds(t *testing.T) {
	examples := map[string][]string{
		"../cmd/worker/.env.example":     RequiredByWorker,
		"../cmd/trigger_ui/.env.example": RequiredByTriggerUI,
		"../cmd/httpin/.env.example":     RequiredByHTTPIn,
	}

	for path, required := range examples {
		contents, err := os.ReadFile(path)
		require.NoError(t, err)

		declared := map[string]bool{}
		for _, line := range strings.Split(string(contents), "\n") {
			line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
			name, _, found := strings.Cut(line, "=")
			if found {
				declared[strings.TrimSpace(name)] = true
			}
		}

		for _, name := range required {
			assert.True(t, declared[name], "%s does not document %s", path, name)
		}
	}
}

func TestOnlyOneExampleFilePerEntrypoint(t *testing.T) {
	strays, err := os.ReadDir("../cmd/trigger_ui")
	require.NoError(t, err)

	for _, entry := range strays {
		assert.NotEqual(t, "example.env", entry.Name(),
			"two example files disagree with each other; .env.example is the one")
	}
}
