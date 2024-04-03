package rclone

import (
	"sync"
	"time"

	"github.com/ansel1/merry/v2"
)

const maxConcurrentTransfers = 5

var queueLock = sync.Mutex{}
var transferQueue = map[Priority][]chan bool{}

func init() {
	for _, priority := range Priorities.Members() {
		transferQueue[priority] = []chan bool{}
	}
}

func waitForTransferSlot(priority Priority, timeout time.Duration) (chan bool, error) {
	ch := make(chan bool)

	queueLock.Lock()
	transferQueue[priority] = append(transferQueue[priority], ch)
	queueLock.Unlock()

	select {
	case <-ch:
		break
	case <-time.After(timeout):
		return nil, merry.Wrap(errTimeout)
	}

	return ch, nil
}

func StartFileTransferQueue() {
	for {
		checkFileTransferQueue()
		time.Sleep(time.Second * 5)
	}
}

func checkFileTransferQueue() {
	stats, _ := GetRcloneStatus()
	count := len(stats.Transferring)

	if count >= maxConcurrentTransfers {
		return
	}

	queueLock.Lock()
	defer queueLock.Unlock()

	for _, priority := range Priorities.Members() {
		started := 0
		for _, ch := range transferQueue[priority] {
			count++
			started++
			ch <- true

			if count >= maxConcurrentTransfers {
				// If we've reached the maximum number of concurrent transfers, then we can stop processing the queue
				// and remove the items that we've already started
				transferQueue[priority] = transferQueue[priority][started:]
				return
			}
		}

		if started > 0 {
			// If we get to here, then we've exhausted the queue for this priority and can replace it with an empty slice
			transferQueue[priority] = []chan bool{}
		}
	}

}
