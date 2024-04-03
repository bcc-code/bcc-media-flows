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

func CheckFileTransferQueue() {
	for {
		stats, _ := GetRcloneStatus()
		count := len(stats.Transferring)

		if count >= maxConcurrentTransfers {
			return
		}

		queueLock.Lock()

		for _, priority := range Priorities.Members() {
			for _, ch := range transferQueue[priority] {
				if count >= maxConcurrentTransfers {
					goto sleep
				}

				count++
				ch <- true
			}
		}

	sleep:
		queueLock.Unlock()
		time.Sleep(time.Second * 5)
	}
}
