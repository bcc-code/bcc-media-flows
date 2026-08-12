package activities

import (
	"context"
	"errors"
	"time"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vizualizer"
	"go.temporal.io/sdk/activity"
)

// Vizualizer exposes activities for the music visualization service.
var Vizualizer *VizualizerActivities

type VizualizerActivities struct {
	Client *vizualizer.Client
}

// SubmitVisualizationArgs are the inputs to submit a visualization job.
// Fields align with vizualizer.CreateVisualizationRequest.
type SubmitVisualizationArgs struct {
	AudioPath    paths.Path
	OutputPath   paths.Path
	Width        int
	Height       int
	FPS          int
	IncludeAudio bool
}

// SubmitVisualization submits a job and returns its JobID.
func (a *VizualizerActivities) SubmitVisualization(ctx context.Context, args SubmitVisualizationArgs) (string, error) {
	req := vizualizer.CreateVisualizationRequest{
		AudioPath:    args.AudioPath.Linux(),
		OutputPath:   args.OutputPath.Linux(),
		Width:        args.Width,
		Height:       args.Height,
		FPS:          args.FPS,
		IncludeAudio: args.IncludeAudio,
	}
	resp, err := a.Client.CreateVisualization(req)
	if err != nil {
		return "", err
	}
	return resp.JobID, nil
}

// WaitForVisualizationArgs controls polling for a given job.
// If PollInterval is zero, defaults to 2s. If Timeout is zero, no timeout.
// If Timeout elapses, returns context.DeadlineExceeded.
// Returns the final job status on success; errors if job failed.
//
// Timeout only bounds the polling. The activity's own HeartbeatTimeout and
// ScheduleToCloseTimeout still apply, and the heartbeat is the shorter of the
// two: GetDefaultActivityOptions sets it to ten minutes.
type WaitForVisualizationArgs struct {
	JobID        string
	PollInterval time.Duration
	Timeout      time.Duration
}

// WaitForVisualization polls until the job completes or fails.
//
// Each poll records a heartbeat. A visualization takes about as long as the
// audio it renders, so without one, any job outlasting the heartbeat timeout is
// killed mid-render and retried into the same wall — the workflow spends its
// whole retry budget failing identically.
func (a *VizualizerActivities) WaitForVisualization(ctx context.Context, args WaitForVisualizationArgs) (*vizualizer.JobStatusResponse, error) {
	if args.JobID == "" {
		return nil, errors.New("JobID is required")
	}
	interval := args.PollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}

	// Apply timeout if specified
	if args.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, args.Timeout)
		defer cancel()
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			status, err := a.Client.GetJob(args.JobID)
			if err != nil {
				return nil, err
			}
			// The status is the heartbeat detail, so a job that is progressing
			// but slow is visible in Temporal rather than just silent.
			activity.RecordHeartbeat(ctx, status)
			switch status.Status {
			case "completed":
				return status, nil
			case "failed":
				return nil, errors.New(status.Message)
			}
		}
	}
}
