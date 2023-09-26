package utils

import (
	"os"

	"github.com/bcc-code/bccm-flows/common"
)

var queue = os.Getenv("QUEUE")

func GetQueue() string {
	if queue != "" {
		return queue
	}
	return common.QueueWorker
}

func GetWorkerQueue() string {
	if queue == common.QueueDebug {
		return common.QueueDebug
	}
	return common.QueueWorker
}

func GetTranscodeQueue() string {
	if queue == common.QueueDebug {
		return common.QueueDebug
	}
	return common.QueueTranscode
}

var isilonPrefix = os.Getenv("ISILON_PREFIX")

func GetIsilonPrefix() string {
	// For local testing
	if isilonPrefix != "" {
		return isilonPrefix
	}
	return "/mnt/isilon"
}
