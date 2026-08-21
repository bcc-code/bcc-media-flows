package rclone

import (
	"context"
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

func waitForTransferSlot(ctx context.Context, priority Priority, timeout time.Duration) error {
	// Create an unbuffered channel
	ch := make(chan bool)

	queueLock.Lock()
	transferQueue[priority] = append(transferQueue[priority], ch)
	queueLock.Unlock()

	select {
	case <-ch:
		break
	case <-ctx.Done():
		removeFromTransferQueue(priority, ch)
		return ctx.Err()
	case <-time.After(timeout):
		removeFromTransferQueue(priority, ch)
		return errTimeout
	}

	return nil
}

func removeFromTransferQueue(priority Priority, ch chan bool) {
	queueLock.Lock()
	defer queueLock.Unlock()
	queue := transferQueue[priority]
	for i, c := range queue {
		if c == ch {
			transferQueue[priority] = append(queue[:i], queue[i+1:]...)
			return
		}
	}
}

func StartFileTransferQueue() {
	for {
		checkFileTransferQueue()
		time.Sleep(time.Second * 5)
	}
}

func checkFileTransferQueue() {
	stats, err := GetRcloneStatus(context.Background())
	if err != nil {
		return
	}

	count := len(stats.Transferring)

	if count >= maxConcurrentTransfers {
		return
	}

	dispatchTransferSlots(count)
}

func dispatchTransferSlots(count int) {
	queueLock.Lock()
	defer queueLock.Unlock()

	for _, priority := range Priorities.Members() {
		var remaining []chan bool
		for i, ch := range transferQueue[priority] {
			if count >= maxConcurrentTransfers {
				remaining = append(remaining, transferQueue[priority][i:]...)
				break
			}

			// This is a non-blocking send, so if nobody is listening on the
			// unbuffered channel we keep the entry for the next round; the
			// waiter removes itself when it gives up.
			select {
			case ch <- true:
				count++
			default:
				remaining = append(remaining, ch)
			}
		}
		transferQueue[priority] = remaining
	}
}
