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
		TaskQueue:              utils.GetWorkerQueue(),
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	logger.Info("Starting WatchFolderTranscode")

	path := params.Path
	path, err := standardizeFileName(ctx, path)
	if err != nil {
		return err
	}
	processingFolder, err := utils.GetSiblingFolder(path, "processing")
	if err != nil {
		return err
	}
	path, err = moveToFolder(ctx, path, processingFolder)
	if err != nil {
		return err
	}
	outFolder, err := utils.GetSiblingFolder(path, "out")
	if err != nil {
		return err
	}
	tmpFolder, err := utils.GetSiblingFolder(path, "tmp")
	if err != nil {
		return err
	}
	errorFolder, err := utils.GetSiblingFolder(path, "error")
	if err != nil {
		return err
	}
	processedFolder, err := utils.GetSiblingFolder(path, "processed")
	if err != nil {
		return err
	}

	var transcodeOutput *activities.EncodeResult
	var transcribeOutput *activities.TranscribeResponse
	ctx = workflow.WithTaskQueue(ctx, utils.GetTranscodeQueue())
	switch params.FolderName {
	case common.FolderProRes422HQHD:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  tmpFolder,
			Resolution: "1920x1080",
			FrameRate:  25,
		}).Get(ctx, &transcodeOutput)
	case common.FolderProRes422HQNative:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:  path,
			OutputDir: tmpFolder,
		}).Get(ctx, &transcodeOutput)
	case common.FolderProRes422HQNative25FPS:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:  path,
			OutputDir: tmpFolder,
			FrameRate: 25,
		}).Get(ctx, &transcodeOutput)
	case common.FolderProRes4444K25FPS:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  tmpFolder,
			Resolution: "3840x2160",
			FrameRate:  25,
		}).Get(ctx, &transcodeOutput)
	case common.FolderAVCIntra100HD:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToH264Activity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  tmpFolder,
			Resolution: "1920x1080",
			FrameRate:  25,
			Bitrate:    "100M",
		}).Get(ctx, &transcodeOutput)
	case common.FolderXDCAMHD422:
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToXDCAMActivity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  tmpFolder,
			Resolution: "1920x1080",
			FrameRate:  25,
			Bitrate:    "60M",
		}).Get(ctx, &transcodeOutput)
	case common.FolderTranscribe:
		ctx = workflow.WithTaskQueue(ctx, utils.GetWorkerQueue())
		err = workflow.ExecuteActivity(ctx, activities.Transcribe, activities.TranscribeParams{
			Language:        "no",
			File:            path,
			DestinationPath: tmpFolder,
		}).Get(ctx, &transcribeOutput)
	default:
		err = fmt.Errorf("codec not supported: %s", params.FolderName)
	}

	ctx = workflow.WithTaskQueue(ctx, utils.GetWorkerQueue())

	if err != nil {
		path, _ = moveToFolder(ctx, path, errorFolder)
		return err
	} else {
		path, _ = moveToFolder(ctx, path, processedFolder)

		if transcodeOutput != nil {
			_, _ = moveToFolder(ctx, transcodeOutput.OutputPath, outFolder)
		}
		if transcribeOutput != nil {
			_, _ = moveToFolder(ctx, transcribeOutput.JSONPath, outFolder)
			_, _ = moveToFolder(ctx, transcribeOutput.SRTPath, outFolder)
			_, _ = moveToFolder(ctx, transcribeOutput.TXTPath, outFolder)
		}
	}

	return nil
}
