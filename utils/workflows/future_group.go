package wfutils

import (
	"go.temporal.io/sdk/workflow"
)

// FutureGroup tracks the futures registered on a workflow.Selector so they can be
// drained exactly, instead of draining a count derived from whatever inputs
// produced them.
//
// Deriving that count is the trap this type exists to remove. Callbacks routinely
// register follow-up futures of their own and return early when something fails,
// so a count computed up front overshoots the moment anything goes wrong, and
// Select then blocks forever waiting on a future that was never registered. Two
// separate workflows shipped that bug (VXExportToVOD and IngestSyncFix), in both
// cases surfacing as an export that hung until its activity timeout and then
// reported a meaningless deadline error rather than the real failure.
//
// For workflow code only: it relies on workflow coroutines being cooperatively
// scheduled on a single goroutine, so it needs no locking.
type FutureGroup struct {
	selector workflow.Selector
	pending  int
}

func NewFutureGroup(ctx workflow.Context) *FutureGroup {
	return &FutureGroup{selector: workflow.NewSelector(ctx)}
}

// Add registers a future along with the callback that handles it once it
// resolves. Callbacks may call Add themselves; those futures get drained by the
// same Wait.
func (g *FutureGroup) Add(future workflow.Future, callback func(f workflow.Future)) {
	g.pending++
	g.selector.AddFuture(future, callback)
}

// Wait blocks until every registered future has resolved, including any that
// callbacks register while draining. Returns immediately if nothing is pending.
func (g *FutureGroup) Wait(ctx workflow.Context) {
	for g.pending > 0 {
		// Select runs exactly one ready callback, which may Add more futures and
		// so push pending back up before we decrement it.
		g.selector.Select(ctx)
		g.pending--
	}
}

// Pending reports how many registered futures have not been handled yet.
func (g *FutureGroup) Pending() int {
	return g.pending
}
