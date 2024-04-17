package environment

import (
	"os"
	"strings"
)

var queue = os.Getenv("QUEUE")

func GetQueue() string {
	if queue != "" {
		return queue
	}
	return QueueWorker
}

func GetWorkerQueue() string {
	if queue == QueueDebug {
		return QueueDebug
	}
	return QueueWorker
}

func GetTranscodeQueue() string {
	if queue == QueueDebug {
		return QueueDebug
	}
	return QueueTranscode
}

func GetAudioQueue() string {
	if queue == QueueDebug {
		return QueueDebug
	}
	return QueueAudio
}

var isilonPrefix = os.Getenv("ISILON_PREFIX")

func GetIsilonPrefix() string {
	// For local testing
	if isilonPrefix != "" {
		return isilonPrefix
	}
	return "/mnt/isilon"
}

var tempMountPrefix = os.Getenv("TEMP_MOUNT_PREFIX")

func GetTempMountPrefix() string {
	// For local testing
	if tempMountPrefix != "" {
		return tempMountPrefix
	}
	return "/mnt/temp"
}

var fileCatalystMountPrefix = os.Getenv("FILECATALYST_MOUNT_PREFIX")

func GetFileCatalystMountPrefix() string {
	// For local testing
	if fileCatalystMountPrefix != "" {
		return fileCatalystMountPrefix
	}
	return "/mnt/filecatalyst"
}

func IsilonPathFix(path string) string {
	return strings.Replace(path, "/mnt/isilon", GetIsilonPrefix(), 1)
}
