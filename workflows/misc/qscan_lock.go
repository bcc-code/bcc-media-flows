package miscworkflows

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/bcc-code/bcc-media-flows/environment"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// QScan runs one job at a time. A job started while another one is analysing
// does not wait: its file ends in file_error "No thread available". So every
// QC run takes the one QScan slot from the QScanLock workflow first, and the
// runs queue there instead of in QScan.

// QScanLockWorkflowID is the fixed id of the single QScanLock execution.
const QScanLockWorkflowID = "qscan_lock"

const (
	qscanAcquireSignal = "qscan_acquire"
	qscanReleaseSignal = "qscan_release"
	qscanGrantedSignal = "qscan_granted"

	// qscanLockIdleTimeout is how long the lock waits for another request
	// before completing. The next request starts it again.
	qscanLockIdleTimeout = time.Minute

	// qscanLockCheckInterval is how often the lock checks that its holder is
	// still running, so a run that was terminated, or failed without
	// releasing, does not keep the slot.
	qscanLockCheckInterval = 5 * time.Minute

	// qscanLockLease bounds one holder. A run gives up on QScan after
	// qscanTimeout, so a holder past this is stuck rather than slow.
	qscanLockLease = qscanTimeout + time.Hour

	// qscanSlotRequestRepeat is how long a run waits for the slot before
	// asking again. The lock ignores a repeated request, but one that comes
	// after the lock completed or was terminated starts it again.
	qscanSlotRequestRepeat = 30 * time.Minute
)

// QScanSlotRequest names the run that wants, or gives back, the QScan slot.
type QScanSlotRequest struct {
	WorkflowID string
	RunID      string
}

// QScanLockState is carried across continue-as-new.
type QScanLockState struct {
	Queue []QScanSlotRequest
}

// QScanLock hands the QScan slot to one run at a time, in the order they ask
// for it. It runs under QScanLockWorkflowID, is started by the first request
// and completes once nobody has asked for qscanLockIdleTimeout.
func QScanLock(ctx workflow.Context, state QScanLockState) error {
	logger := workflow.GetLogger(ctx)
	acquireCh := workflow.GetSignalChannel(ctx, qscanAcquireSignal)
	releaseCh := workflow.GetSignalChannel(ctx, qscanReleaseSignal)

	queue := state.Queue
	enqueue := func(req QScanSlotRequest) {
		if !slices.Contains(queue, req) {
			queue = append(queue, req)
		}
	}
	drain := func() {
		for {
			var req QScanSlotRequest
			if !acquireCh.ReceiveAsync(&req) {
				break
			}
			enqueue(req)
		}
		// Nobody holds the slot here, so any release is late and means nothing.
		for releaseCh.ReceiveAsync(nil) {
		}
	}

	for {
		drain()

		if workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
			logger.Info("History is long, continuing as new", "queued", len(queue))
			return workflow.NewContinueAsNewError(ctx, QScanLock, QScanLockState{Queue: queue})
		}

		if len(queue) == 0 {
			var req QScanSlotRequest
			if ok, _ := acquireCh.ReceiveWithTimeout(ctx, qscanLockIdleTimeout, &req); ok {
				enqueue(req)
				continue
			}
			drain()
			if len(queue) == 0 {
				return nil
			}
			continue
		}

		holder := queue[0]
		queue = queue[1:]

		err := workflow.SignalExternalWorkflow(ctx, holder.WorkflowID, holder.RunID, qscanGrantedSignal, nil).Get(ctx, nil)
		if err != nil {
			logger.Warn("Could not hand the QScan slot to a run, skipping it", "workflowID", holder.WorkflowID, "runID", holder.RunID, "error", err)
			continue
		}
		logger.Info("QScan slot granted", "workflowID", holder.WorkflowID, "runID", holder.RunID, "queued", len(queue))

		holdQScanSlot(ctx, holder, acquireCh, releaseCh, enqueue)

		// A request the holder repeated while it waited may still be queued.
		queue = slices.DeleteFunc(queue, func(r QScanSlotRequest) bool { return r == holder })
	}
}

// holdQScanSlot returns once the holder releases the slot, has ended, or has
// held it past qscanLockLease. Requests that arrive meanwhile are queued.
func holdQScanSlot(
	ctx workflow.Context,
	holder QScanSlotRequest,
	acquireCh, releaseCh workflow.ReceiveChannel,
	enqueue func(QScanSlotRequest),
) {
	logger := workflow.GetLogger(ctx)
	grantedAt := workflow.Now(ctx)

	for {
		released, checkDue := false, false

		timerCtx, cancelTimer := workflow.WithCancel(ctx)
		selector := workflow.NewSelector(ctx)
		selector.AddReceive(releaseCh, func(c workflow.ReceiveChannel, _ bool) {
			var req QScanSlotRequest
			c.Receive(ctx, &req)
			if req == holder {
				released = true
				return
			}
			logger.Info("Ignoring a QScan release from a run that does not hold the slot", "workflowID", req.WorkflowID, "runID", req.RunID)
		})
		selector.AddReceive(acquireCh, func(c workflow.ReceiveChannel, _ bool) {
			var req QScanSlotRequest
			c.Receive(ctx, &req)
			if req != holder {
				enqueue(req)
			}
		})
		selector.AddFuture(workflow.NewTimer(timerCtx, qscanLockCheckInterval), func(workflow.Future) {
			checkDue = true
		})
		selector.Select(ctx)
		cancelTimer()

		if released {
			logger.Info("QScan slot released", "workflowID", holder.WorkflowID, "runID", holder.RunID)
			return
		}
		if !checkDue {
			continue
		}

		if workflow.Now(ctx).Sub(grantedAt) >= qscanLockLease {
			logger.Warn("QScan slot held past its lease, taking it back", "workflowID", holder.WorkflowID, "runID", holder.RunID, "lease", qscanLockLease)
			return
		}

		running, err := qscanSlotHolderRunning(ctx, holder)
		if err != nil {
			logger.Warn("Could not check the QScan slot holder, keeping it", "workflowID", holder.WorkflowID, "runID", holder.RunID, "error", err)
			continue
		}
		if !running {
			logger.Warn("QScan slot holder ended without releasing, taking the slot back", "workflowID", holder.WorkflowID, "runID", holder.RunID)
			return
		}
	}
}

func qscanSlotHolderRunning(ctx workflow.Context, holder QScanSlotRequest) (bool, error) {
	ctx = workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
		ScheduleToCloseTimeout: time.Minute,
		RetryPolicy:            &temporal.RetryPolicy{MaximumAttempts: 3},
	})
	var running bool
	err := workflow.ExecuteLocalActivity(ctx, QScanSlotHolderRunning, holder).Get(ctx, &running)
	return running, err
}

// QScanSlotHolderRunning reports whether the run still exists and is running.
func QScanSlotHolderRunning(ctx context.Context, holder QScanSlotRequest) (bool, error) {
	c, ok := ctx.Value(ClientContextKey).(client.Client)
	if !ok {
		return false, errors.New("no temporal client in the activity context")
	}
	resp, err := c.DescribeWorkflowExecution(ctx, holder.WorkflowID, holder.RunID)
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("describing %s: %w", holder.WorkflowID, err)
	}
	return resp.GetWorkflowExecutionInfo().GetStatus() == enums.WORKFLOW_EXECUTION_STATUS_RUNNING, nil
}

// RequestQScanSlot queues the run at the QScanLock, starting the lock if it is
// not running.
func RequestQScanSlot(ctx context.Context, req QScanSlotRequest) error {
	c, ok := ctx.Value(ClientContextKey).(client.Client)
	if !ok {
		return errors.New("no temporal client in the activity context")
	}
	_, err := c.SignalWithStartWorkflow(ctx, QScanLockWorkflowID, qscanAcquireSignal, req,
		client.StartWorkflowOptions{
			ID:                    QScanLockWorkflowID,
			TaskQueue:             environment.GetWorkerQueue(),
			TypedSearchAttributes: wfutils.TypedSearchAttributes("", "system"),
		},
		QScanLock, QScanLockState{},
	)
	if err != nil {
		return fmt.Errorf("signal-with-start %s: %w", QScanLockWorkflowID, err)
	}
	return nil
}

// acquireQScanSlot waits until this run holds the QScan slot. The returned
// release gives it back; it is safe to call more than once. If release is
// never called, the lock takes the slot back once this run ends.
func acquireQScanSlot(ctx workflow.Context) (release func(), err error) {
	logger := workflow.GetLogger(ctx)
	execution := workflow.GetInfo(ctx).WorkflowExecution
	req := QScanSlotRequest{WorkflowID: execution.ID, RunID: execution.RunID}

	requestCtx := workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    time.Minute,
		},
	})
	granted := workflow.GetSignalChannel(ctx, qscanGrantedSignal)
	start := workflow.Now(ctx)

	for {
		if err := workflow.ExecuteLocalActivity(requestCtx, RequestQScanSlot, req).Get(ctx, nil); err != nil {
			return nil, fmt.Errorf("requesting the QScan slot: %w", err)
		}
		if ok, _ := granted.ReceiveWithTimeout(ctx, qscanSlotRequestRepeat, nil); ok {
			break
		}
		logger.Info("Still waiting for the QScan slot, asking again", "waited", workflow.Now(ctx).Sub(start))
	}
	logger.Info("Holding the QScan slot", "waited", workflow.Now(ctx).Sub(start))

	released := false
	return func() {
		if released {
			return
		}
		released = true
		// Released on cancellation too, so the next run is not left waiting.
		releaseCtx, _ := workflow.NewDisconnectedContext(ctx)
		err := workflow.SignalExternalWorkflow(releaseCtx, QScanLockWorkflowID, "", qscanReleaseSignal, req).Get(releaseCtx, nil)
		if err != nil {
			logger.Warn("Could not release the QScan slot; the lock takes it back once this run ends", "error", err)
		}
	}, nil
}
