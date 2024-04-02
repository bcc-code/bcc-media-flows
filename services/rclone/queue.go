package rclone

import (
	"github.com/ansel1/merry/v2"
	"sync"
	"time"
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

func CheckFileTransferQueue() {
	for {
		stats, _ := GetRcloneStatus()
		count := len(stats.Transferring)

		if count >= maxConcurrentTransfers {
			return
		}

		for _, priority := range Priorities.Members() {
			for _, ch := range transferQueue[priority] {
				if count >= maxConcurrentTransfers {
					return
				}
				ch <- true
			}
		}
		time.Sleep(time.Second * 5)
	}
}
