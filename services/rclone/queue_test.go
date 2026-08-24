package rclone

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func queueLen(priority Priority) int {
	queueLock.Lock()
	defer queueLock.Unlock()
	return len(transferQueue[priority])
}

func resetTransferQueue() {
	queueLock.Lock()
	defer queueLock.Unlock()
	for _, priority := range Priorities.Members() {
		transferQueue[priority] = []chan bool{}
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, cond())
}

func TestWaitForTransferSlotRemovesAbandonedWaiters(t *testing.T) {
	resetTransferQueue()
	defer resetTransferQueue()

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := waitForTransferSlot(ctx, PriorityNormal, time.Minute)
			assert.ErrorIs(t, err, context.Canceled)
		}()
	}

	waitFor(t, func() bool { return queueLen(PriorityNormal) == 3 })
	cancel()
	wg.Wait()

	assert.Equal(t, 0, queueLen(PriorityNormal))

	dispatchTransferSlots(0)
	assert.Equal(t, 0, queueLen(PriorityNormal))
}

func TestWaitForTransferSlotRemovesTimedOutWaiters(t *testing.T) {
	resetTransferQueue()
	defer resetTransferQueue()

	err := waitForTransferSlot(context.Background(), PriorityLow, 10*time.Millisecond)
	assert.ErrorIs(t, err, errTimeout)
	assert.Equal(t, 0, queueLen(PriorityLow))
}

func TestDispatchTransferSlotsKeepsUnservedWaiters(t *testing.T) {
	resetTransferQueue()
	defer resetTransferQueue()

	served := make(chan error, 3)
	for i := 0; i < 3; i++ {
		go func() {
			served <- waitForTransferSlot(context.Background(), PriorityHigh, 5*time.Second)
		}()
	}

	waitFor(t, func() bool { return queueLen(PriorityHigh) == 3 })

	waitFor(t, func() bool {
		dispatchTransferSlots(maxConcurrentTransfers - 1)
		return queueLen(PriorityHigh) == 2
	})
	require.NoError(t, <-served)

	waitFor(t, func() bool {
		dispatchTransferSlots(0)
		return queueLen(PriorityHigh) == 0
	})
	require.NoError(t, <-served)
	require.NoError(t, <-served)
}
