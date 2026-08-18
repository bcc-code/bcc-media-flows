package environment

import (
	"strings"
)

func GetQueue() string {
	if q := Get().Queue; q != "" {
		return q
	}
	return QueueWorker
}

// queueOrDebug collapses a specialised queue onto the debug one. A debug worker polls
// only QueueDebug, so an activity routed to worker, transcode, audio or live would sit
// unscheduled unless it lands there too.
func queueOrDebug(queue string) string {
	if Get().Queue == QueueDebug {
		return QueueDebug
	}
	return queue
}

func GetWorkerQueue() string { return queueOrDebug(QueueWorker) }

func GetTranscodeQueue() string { return queueOrDebug(QueueTranscode) }

func GetAudioQueue() string { return queueOrDebug(QueueAudio) }

func GetLiveIngestQueue() string { return queueOrDebug(QueueLiveIngest) }

func GetIsilonPrefix() string {
	// For local testing
	if prefix := Get().Paths.IsilonPrefix(); prefix != "" {
		return prefix
	}
	return "/mnt/isilon"
}

func GetTempMountPrefix() string {
	// For local testing
	if prefix := Get().Paths.TempMount(); prefix != "" {
		return prefix
	}
	return "/mnt/temp"
}

func GetFileCatalystMountPrefix() string {
	// For local testing
	if prefix := Get().Paths.FileCatalystMount(); prefix != "" {
		return prefix
	}
	return "/mnt/filecatalyst"
}

func IsilonPathFix(path string) string {
	return strings.Replace(path, "/mnt/isilon", GetIsilonPrefix(), 1)
}
