package environment_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every environment read belongs in the environment package, so what a process is
// configured with is one file rather than a search.
func TestOnlyTheEnvironmentPackageReadsTheEnvironment(t *testing.T) {
	var offenders []string

	err := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if name := info.Name(); name == ".git" || name == ".claude" || name == "environment" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(contents), "os.Getenv(") {
			offenders = append(offenders, path)
		}
		return nil
	})
	require.NoError(t, err)

	assert.Empty(t, offenders, "read these through environment.Get() instead")
}
