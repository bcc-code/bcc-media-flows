package utils

import (
	"github.com/bcc-code/bccm-flows/common"
	"os"
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
