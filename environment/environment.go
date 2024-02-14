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

func GetLowPriorityQueue() string {
	if queue == QueueDebug {
		return QueueDebug
	}
	return QueueLowPriority
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

var dmzShareMountPrefix = os.Getenv("DMZSHARE_MOUNT_PREFIX")

func GetDmzShareMountPrefix() string {
	// For local testing
	if dmzShareMountPrefix != "" {
		return dmzShareMountPrefix
	}
	return "/mnt/dmzshare"
}

func IsilonPathFix(path string) string {
	return strings.Replace(path, "/mnt/isilon", GetIsilonPrefix(), 1)
}
