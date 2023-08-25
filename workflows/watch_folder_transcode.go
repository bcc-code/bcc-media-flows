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
			MaximumAttempts: 1,
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
	path, err := utils.MoveToParentFolder(path, "processing")
	if err != nil {
		return err
	}
	outFolder, err := utils.GetSiblingFolder(path, "tmp")
	if err != nil {
		return err
	}

	var output activities.EncodeResult
	ctx = workflow.WithTaskQueue(ctx, common.QueueTranscode)
	switch params.ToCodec {
	case common.CodecProRes422HQHD:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  outFolder,
			Resolution: "1920x1080",
			FrameRate:  25,
		}).Get(ctx, &output)
	case common.CodecProRes422HQNative:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:  path,
			OutputDir: outFolder,
		}).Get(ctx, &output)
	case common.CodecProRes422HQNative25FPS:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:  path,
			OutputDir: outFolder,
			FrameRate: 25,
		}).Get(ctx, &output)
	case common.CodecProRes4444K25FPS:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  outFolder,
			Resolution: "3840x2160",
			FrameRate:  25,
		}).Get(ctx, &output)
	case common.CodecAVCIntra100HD:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToH264Activity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  outFolder,
			Resolution: "1920x1080",
			FrameRate:  25,
			Bitrate:    "100M",
		}).Get(ctx, &output)
	case common.CodecXDCAMHD422:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToXDCAMActivity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  outFolder,
			Resolution: "1920x1080",
			FrameRate:  25,
		}).Get(ctx, &output)
	default:
		err = fmt.Errorf("codec not supported: %s", params.ToCodec)
	}

	if err != nil {
		path, _ = utils.MoveToParentFolder(path, "error")
		return err
	} else {
		path, _ = utils.MoveToParentFolder(path, "processed")

		_, _ = utils.MoveToParentFolder(output.OutputPath, "out")
	}

	return nil
}
