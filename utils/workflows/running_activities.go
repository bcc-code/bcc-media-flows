package wfutils

import (
	"sync"
	"sync/atomic"
)

// RunningActivities counts the activities executing in this process. The
// activity interceptor maintains it, so it reflects the worker that is doing
// the work rather than the worker that scheduled it — the two are usually
// different, since Execute routes each activity to the queue that owns it.
//
// cmd/worker uses it to hold back the self-update while the worker is busy.
var RunningActivities = &ActivityCounter{}

// ActivityCounter counts in-flight activities.
//
// A sync.WaitGroup is the obvious fit and the wrong one: Add must not race
// with Wait once the counter has reached zero, and activities start at
// arbitrary times relative to whoever is reading the count, which panics with
// "WaitGroup misuse: Add called concurrently with Wait".
type ActivityCounter struct {
	running atomic.Int64
}

// Started records the start of an activity and returns the function that
// records its end. The returned function is safe to call more than once, so it
// can be deferred without any further guard.
func (c *ActivityCounter) Started() func() {
	c.running.Add(1)

	var once sync.Once
	return func() {
		once.Do(func() {
			c.running.Add(-1)
		})
	}
}

// Running reports how many activities are executing right now.
//
// The answer is only true for the instant it is read: the worker can pick up
// another task immediately afterwards. Callers that must not interrupt work in
// progress have to stop the worker rather than trust this.
func (c *ActivityCounter) Running() int {
	return int(c.running.Load())
}
