package utils_test

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runWithDeadline runs fn and fails the test if it has not returned in time, rather
// than letting a deadlock stall the whole package until the go test timeout.
func runWithDeadline(t *testing.T, what string, fn func()) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("%s did not return within 30s — it deadlocked", what)
	}
}

// chattyStderrCmd builds a command that writes more than a pipe buffer's worth of
// stderr before writing anything to stdout and exiting, then prints a JSON object on
// stderr the way ffmpeg's loudnorm filter does.
//
// 2000 lines of ~64 bytes is ~128 KiB, comfortably past the 64 KiB pipe buffer, so
// the child blocks in write() unless stderr is being drained concurrently.
func chattyStderrCmd() *exec.Cmd {
	script := `
i=0
while [ $i -lt 2000 ]; do
  echo "[mpeg4 @ 0x0] warning: concealing errors in frame, dts non-monotonic" >&2
  i=$((i+1))
done
echo "{" >&2
echo "	\"input_i\" : \"-23.05\"," >&2
echo "	\"input_tp\" : \"-6.20\"," >&2
echo "	\"input_lra\" : \"5.30\"" >&2
echo "}" >&2
echo "progress=end"
`
	return exec.Command("sh", "-c", script)
}

// The regression. Reading stdout to EOF and only then reading stderr deadlocks once
// the child fills the stderr pipe: it blocks in write(), so it never exits, so
// stdout never reaches EOF.
func TestExecuteAnalysisCmd_ChattyStderrDoesNotDeadlock(t *testing.T) {
	var result string
	var err error

	runWithDeadline(t, "ExecuteAnalysisCmd with 128 KiB of stderr", func() {
		result, err = utils.ExecuteAnalysisCmd(chattyStderrCmd(), nil)
	})

	require.NoError(t, err)

	// The JSON has to survive being buried in all that noise.
	var parsed map[string]string
	require.NoError(t, json.Unmarshal([]byte(result), &parsed),
		"expected a parseable JSON object, got:\n%s", result)
	assert.Equal(t, "-23.05", parsed["input_i"])
	assert.Equal(t, "-6.20", parsed["input_tp"])
	assert.Equal(t, "5.30", parsed["input_lra"])
}

// The quiet case, which is what a clean media file looks like.
func TestExecuteAnalysisCmd_ExtractsJSONFromQuietStderr(t *testing.T) {
	cmd := exec.Command("sh", "-c",
		`echo "Input #0, mov,mp4" >&2; echo "{" >&2; echo "	\"input_i\" : \"-18.00\"" >&2; echo "}" >&2; echo "progress=end"`)

	result, err := utils.ExecuteAnalysisCmd(cmd, nil)
	require.NoError(t, err)

	var parsed map[string]string
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	assert.Equal(t, "-18.00", parsed["input_i"])
}

// Progress lines on stdout still reach the callback.
func TestExecuteAnalysisCmd_ForwardsStdoutToCallback(t *testing.T) {
	cmd := exec.Command("sh", "-c",
		`echo "progress=continue"; echo "progress=end"; echo "{" >&2; echo "}" >&2`)

	var lines []string
	_, err := utils.ExecuteAnalysisCmd(cmd, func(line string) {
		lines = append(lines, line)
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"progress=continue", "progress=end"}, lines)
}

// A failing command must report why. Previously the stderr content was dropped and
// callers saw a bare "exit status 1".
func TestExecuteAnalysisCmd_FailureIncludesStderr(t *testing.T) {
	cmd := exec.Command("sh", "-c", `echo "No such file or directory" >&2; exit 1`)

	_, err := utils.ExecuteAnalysisCmd(cmd, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "No such file or directory")
}

// Stderr from a failure is bounded, because activity errors are stored in the
// Temporal workflow history.
func TestExecuteAnalysisCmd_FailureStderrIsTruncated(t *testing.T) {
	cmd := exec.Command("sh", "-c",
		`i=0; while [ $i -lt 5000 ]; do echo "an extremely repetitive decode warning" >&2; i=$((i+1)); done; exit 1`)

	// Also past the pipe buffer, so this needs the deadline guard too.
	var err error
	runWithDeadline(t, "ExecuteAnalysisCmd with a failing chatty command", func() {
		_, err = utils.ExecuteAnalysisCmd(cmd, nil)
	})

	require.Error(t, err)
	assert.Less(t, len(err.Error()), 8192, "error message should be bounded, not the whole stderr stream")
	assert.Contains(t, err.Error(), "truncated")
}

// ExecuteCmd buffers stderr already, so it never had the deadlock. Cover it so the
// two stay consistent.
func TestExecuteCmd_ChattyStderrDoesNotDeadlock(t *testing.T) {
	script := fmt.Sprintf(`
i=0
while [ $i -lt 2000 ]; do
  echo "%s" >&2
  i=$((i+1))
done
echo "done"
`, strings.Repeat("x", 60))

	var result string
	var err error

	runWithDeadline(t, "ExecuteCmd with 128 KiB of stderr", func() {
		result, err = utils.ExecuteCmd(exec.Command("sh", "-c", script), nil)
	})

	require.NoError(t, err)
	assert.Equal(t, "done\n", result)
}

func TestExecuteCmd_FailureIncludesStderr(t *testing.T) {
	cmd := exec.Command("sh", "-c", `echo "boom" >&2; exit 2`)

	_, err := utils.ExecuteCmd(cmd, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
