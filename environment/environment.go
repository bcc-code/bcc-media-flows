package environment

import (
	"os"
	"strings"
)

// queue is read per call rather than once at init, so a .env loaded in main and a
// t.Setenv in a test both take effect.
func queue() string {
	return os.Getenv("QUEUE")
}

func GetQueue() string {
	if q := queue(); q != "" {
		return q
	}
	return QueueWorker
}

func GetWorkerQueue() string {
	if queue() == QueueDebug {
		return QueueDebug
	}
	return QueueWorker
}

func GetTranscodeQueue() string {
	if queue() == QueueDebug {
		return QueueDebug
	}
	return QueueTranscode
}

func GetAudioQueue() string {
	if queue() == QueueDebug {
		return QueueDebug
	}
	return QueueAudio
}

func GetLiveIngestQueue() string {
	if queue() == QueueDebug {
		return QueueDebug
	}
	return QueueLiveIngest
}

func GetIsilonPrefix() string {
	// For local testing
	if prefix := os.Getenv("ISILON_PREFIX"); prefix != "" {
		return prefix
	}
	return "/mnt/isilon"
}

func GetTempMountPrefix() string {
	// For local testing
	if prefix := os.Getenv("TEMP_MOUNT_PREFIX"); prefix != "" {
		return prefix
	}
	return "/mnt/temp"
}

func GetFileCatalystMountPrefix() string {
	// For local testing
	if prefix := os.Getenv("FILECATALYST_MOUNT_PREFIX"); prefix != "" {
		return prefix
	}
	return "/mnt/filecatalyst"
}

func IsilonPathFix(path string) string {
	return strings.Replace(path, "/mnt/isilon", GetIsilonPrefix(), 1)
}
