package miscworkflows

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/bcc-code/bcc-media-flows/environment"
)

// The histories in testdata/replay were recorded from the transcribe workflows before
// Transcribe moved to its own struct and queue, with every activity on the debug queue.
// A history exported from production (`temporal workflow show -w <id> --output json`)
// can be dropped in beside them.
const replayDir = "testdata/replay"

func replayHistories(t *testing.T) map[string]*historypb.History {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(replayDir, "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "no histories in %s", replayDir)

	histories := map[string]*historypb.History{}
	for _, file := range files {
		f, err := os.Open(file)
		require.NoError(t, err)
		history, err := client.HistoryFromJSON(f, client.HistoryJSONOptions{})
		_ = f.Close()
		require.NoError(t, err, file)
		histories[filepath.Base(file)] = history
	}

	return histories
}

func replay(t *testing.T, history *historypb.History) error {
	t.Helper()

	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflow(TranscribeFile)
	replayer.RegisterWorkflow(TranscribeVX)

	return replayer.ReplayWorkflowHistory(nil, history)
}

func withTranscribeQueue(t *testing.T, enabled string) {
	t.Helper()

	t.Cleanup(func() { environment.Load() })
	t.Setenv("TRANSCRIBE_QUEUE_ENABLED", enabled)
	environment.Load()
}

// Running workflows must replay against the new code whichever way the flag is set:
// with it on, Transcribe is scheduled on the transcribe queue against a history that
// recorded another one, which is what flipping it mid-workflow does.
func TestTranscribeWorkflowsReplay(t *testing.T) {
	histories := replayHistories(t)

	for _, enabled := range []string{"false", "true"} {
		for name, history := range histories {
			t.Run("TRANSCRIBE_QUEUE_ENABLED="+enabled+"/"+name, func(t *testing.T) {
				withTranscribeQueue(t, enabled)

				require.NoError(t, replay(t, history))
			})
		}
	}
}

// Guards the test above against passing vacuously: a history whose Transcribe was
// recorded under another name has to fail to replay.
func TestTranscribeWorkflowsReplay_CatchesARenamedActivity(t *testing.T) {
	for name, history := range replayHistories(t) {
		t.Run(name, func(t *testing.T) {
			renamed := 0
			for _, event := range history.GetEvents() {
				scheduled := event.GetActivityTaskScheduledEventAttributes()
				if scheduled.GetActivityType().GetName() == "Transcribe" {
					scheduled.ActivityType.Name = "TranscribeRenamed"
					renamed++
				}
			}
			require.Equal(t, 1, renamed, "the history should schedule Transcribe once")

			require.Error(t, replay(t, history))
		})
	}
}
