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
	Path       string
	FolderName string
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
	path, err := utils.FixFilename(path)
	if err != nil {
		return err
	}
	path, err = utils.MoveToParentFolder(path, "processing")
	if err != nil {
		return err
	}
	outFolder, err := utils.GetSiblingFolder(path, "tmp")
	if err != nil {
		return err
	}

	var transcodeOutput *activities.EncodeResult
	var transcribeOutput *activities.TranscribeResponse
	ctx = workflow.WithTaskQueue(ctx, common.QueueTranscode)
	switch params.FolderName {
	case common.FolderProRes422HQHD:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  outFolder,
			Resolution: "1920x1080",
			FrameRate:  25,
		}).Get(ctx, &transcodeOutput)
	case common.FolderProRes422HQNative:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:  path,
			OutputDir: outFolder,
		}).Get(ctx, &transcodeOutput)
	case common.FolderProRes422HQNative25FPS:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:  path,
			OutputDir: outFolder,
			FrameRate: 25,
		}).Get(ctx, &transcodeOutput)
	case common.FolderProRes4444K25FPS:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  outFolder,
			Resolution: "3840x2160",
			FrameRate:  25,
		}).Get(ctx, &transcodeOutput)
	case common.FolderAVCIntra100HD:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToH264Activity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  outFolder,
			Resolution: "1920x1080",
			FrameRate:  25,
			Bitrate:    "100M",
		}).Get(ctx, &transcodeOutput)
	case common.FolderXDCAMHD422:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToXDCAMActivity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  outFolder,
			Resolution: "1920x1080",
			FrameRate:  25,
			Bitrate:    "60M",
		}).Get(ctx, &transcodeOutput)
	case common.FolderTranscribe:
		ctx = workflow.WithTaskQueue(ctx, common.QueueWorker)
		err = workflow.ExecuteActivity(ctx, activities.Transcribe, activities.TranscribeParams{
			Language:        "no",
			File:            path,
			DestinationPath: outFolder,
		}).Get(ctx, &transcribeOutput)
	default:
		err = fmt.Errorf("codec not supported: %s", params.FolderName)
	}

	if err != nil {
		path, _ = utils.MoveToParentFolder(path, "error")
		return err
	} else {
		path, _ = utils.MoveToParentFolder(path, "processed")

		if transcodeOutput != nil {
			_, _ = utils.MoveToParentFolder(transcodeOutput.OutputPath, "out")
		}
		if transcribeOutput != nil {
			_, _ = utils.MoveToParentFolder(transcribeOutput.JSONPath, "out")
			_, _ = utils.MoveToParentFolder(transcribeOutput.SRTPath, "out")
			_, _ = utils.MoveToParentFolder(transcribeOutput.TXTPath, "out")
		}
	}

	return nil
}
