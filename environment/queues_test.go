package environment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func withQueue(t *testing.T, queue string) {
	t.Helper()
	t.Cleanup(func() { Load() })
	t.Setenv("QUEUE", queue)
	Load()
}

func TestGetQueue_UnsetIsTheWorkerQueue(t *testing.T) {
	withQueue(t, "")

	assert.Equal(t, QueueWorker, GetQueue())
}

func TestGetQueue_ReturnsWhatIsConfigured(t *testing.T) {
	withQueue(t, QueueLowPriority)

	assert.Equal(t, QueueLowPriority, GetQueue())
}

func TestDebugCollapsesEverySpecialisedQueue(t *testing.T) {
	withQueue(t, QueueDebug)

	assert.Equal(t, QueueDebug, GetQueue())
	assert.Equal(t, QueueDebug, GetWorkerQueue())
	assert.Equal(t, QueueDebug, GetTranscodeQueue())
	assert.Equal(t, QueueDebug, GetAudioQueue())
	assert.Equal(t, QueueDebug, GetLiveIngestQueue())
}

func TestEachSpecialisedQueueKeepsItsOwnNameOutsideDebug(t *testing.T) {
	withQueue(t, QueueLowPriority)

	assert.Equal(t, QueueWorker, GetWorkerQueue())
	assert.Equal(t, QueueTranscode, GetTranscodeQueue())
	assert.Equal(t, QueueAudio, GetAudioQueue())
	assert.Equal(t, QueueLiveIngest, GetLiveIngestQueue())
}

func TestASpecialisedQueueIsNotTheConfiguredOne(t *testing.T) {
	withQueue(t, QueueAudio)

	assert.Equal(t, QueueAudio, GetQueue())
	assert.Equal(t, QueueWorker, GetWorkerQueue())
	assert.Equal(t, QueueTranscode, GetTranscodeQueue())
}
