package batonactivities

import (
	"context"
	"time"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/baton"
	"go.temporal.io/sdk/activity"
)

type QCParams struct {
	Path paths.Path
	Plan baton.TestPlan
}

func QC(ctx context.Context, input QCParams) (*baton.QCReport, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting BatonQCActivity")

	client := getClient()

	r, err := baton.StartTask(client, input.Path, input.Plan)
	if err != nil {
		return nil, err
	}

	logger.Info("Baton QC started")
	progress, err := baton.GetTaskProgress(client, r.TaskID)
	for err == nil && progress.Progress < 100 {
		activity.RecordHeartbeat(ctx, progress)
		time.Sleep(time.Second * 10)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		progress, err = baton.GetTaskProgress(client, r.TaskID)
	}

	if err != nil {
		return nil, err
	}

	result, err := baton.GetQCReport(client, r.TaskID)
	if err != nil {
		return nil, err
	}

	return result, nil
}
