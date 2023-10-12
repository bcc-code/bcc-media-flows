package batonactivities

import (
	"context"
	"github.com/bcc-code/bccm-flows/services/baton"
	"github.com/bcc-code/bccm-flows/utils"
	"go.temporal.io/sdk/activity"
	"time"
)

type QCParams struct {
	Path utils.Path
	Plan baton.TestPlan
}

type QCResult struct {
}

func QC(ctx context.Context, input *QCParams) (*QCResult, error) {
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
}
