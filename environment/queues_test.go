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
	assert.Equal(t, QueueDebug, GetTranscribeQueue())
}

func TestEachSpecialisedQueueKeepsItsOwnNameOutsideDebug(t *testing.T) {
	withQueue(t, QueueLowPriority)

	assert.Equal(t, QueueWorker, GetWorkerQueue())
	assert.Equal(t, QueueTranscode, GetTranscodeQueue())
	assert.Equal(t, QueueAudio, GetAudioQueue())
	assert.Equal(t, QueueLiveIngest, GetLiveIngestQueue())
	assert.Equal(t, QueueTranscribe, GetTranscribeQueue())
}

func TestASpecialisedQueueIsNotTheConfiguredOne(t *testing.T) {
	withQueue(t, QueueAudio)

	assert.Equal(t, QueueAudio, GetQueue())
	assert.Equal(t, QueueWorker, GetWorkerQueue())
	assert.Equal(t, QueueTranscode, GetTranscodeQueue())
}

func TestTranscribeQueueEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "unset is off", value: "", want: false},
		{name: "true is on", value: "true", want: true},
		{name: "1 is on", value: "1", want: true},
		{name: "false is off", value: "false", want: false},
		{name: "garbage falls back to off", value: "yes please", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Cleanup(func() { Load() })
			t.Setenv("TRANSCRIBE_QUEUE_ENABLED", test.value)

			assert.Equal(t, test.want, Load().TranscribeQueueEnabled)
		})
	}
}
