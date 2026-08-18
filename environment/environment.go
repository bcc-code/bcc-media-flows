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

func GetWorkerQueue() string {
	if Get().Queue == QueueDebug {
		return QueueDebug
	}
	return QueueWorker
}

func GetTranscodeQueue() string {
	if Get().Queue == QueueDebug {
		return QueueDebug
	}
	return QueueTranscode
}

func GetAudioQueue() string {
	if Get().Queue == QueueDebug {
		return QueueDebug
	}
	return QueueAudio
}

func GetLiveIngestQueue() string {
	if Get().Queue == QueueDebug {
		return QueueDebug
	}
	return QueueLiveIngest
}

func GetIsilonPrefix() string {
	// For local testing
	if prefix := Get().IsilonPrefix; prefix != "" {
		return prefix
	}
	return "/mnt/isilon"
}

func GetTempMountPrefix() string {
	// For local testing
	if prefix := Get().TempMountPrefix; prefix != "" {
		return prefix
	}
	return "/mnt/temp"
}

func GetFileCatalystMountPrefix() string {
	// For local testing
	if prefix := Get().FileCatalystMountPrefix; prefix != "" {
		return prefix
	}
	return "/mnt/filecatalyst"
}

func IsilonPathFix(path string) string {
	return strings.Replace(path, "/mnt/isilon", GetIsilonPrefix(), 1)
}
