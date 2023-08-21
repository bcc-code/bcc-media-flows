package workflows

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/utils"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"time"
)

type WatchFolderTranscodeInput struct {
	Path    string
	ToCodec string
}

func WatchFolderTranscode(ctx workflow.Context, params WatchFolderTranscodeInput) error {
	logger := workflow.GetLogger(ctx)
	options := workflow.ActivityOptions{
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Minute * 1,
			MaximumAttempts: 10,
			MaximumInterval: time.Hour * 1,
		},
		StartToCloseTimeout:    time.Hour * 4,
		ScheduleToCloseTimeout: time.Hour * 48,
		HeartbeatTimeout:       time.Minute * 1,
		TaskQueue:              common.QueueWorker,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	logger.Info("Starting WatchFolderTranscode")

	path := params.Path
	path = utils.MoveToSiblingFolder(path, "processing")

	var err error

	switch params.ToCodec {
	case common.CodecProRes422HQ_HD:
		ctx = workflow.WithTaskQueue(ctx, common.QueueTranscode)
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToProResActivity, activities.TranscodeToProResParams{
			FilePath:  path,
			OutputDir: utils.GetSiblingFolder(path, "out"),
		}).Get(ctx, nil)
	default:
		err = fmt.Errorf("codec not supported: %s", params.ToCodec)
	}

	if err != nil {
		path = utils.MoveToSiblingFolder(path, "error")
		return err
	} else {
		path = utils.MoveToSiblingFolder(path, "processed")
	}

	return nil
}
