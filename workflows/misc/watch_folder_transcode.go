package miscworkflows

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type WatchFolderTranscodeInput struct {
	Path       string
	FolderName string
}

// WatchFolderTranscode is a flow triggered by a file watcher watching for changes at the configured paths.
func WatchFolderTranscode(ctx workflow.Context, params WatchFolderTranscodeInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting WatchFolderTranscode")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	path := paths.MustParse(params.Path)
	dir := path.Dir()

	path, err := wfutils.StandardizeFileName(ctx, path)
	if err != nil {
		return err
	}
	processingFolder := dir.Append("../processing")
	err = wfutils.CreateFolder(ctx, processingFolder)
	if err != nil {
		return err
	}
	path, err = wfutils.MoveToFolder(ctx, path, processingFolder, rclone.PriorityNormal)
	if err != nil {
		return err
	}
	outFolder := dir.Append("../out")
	err = wfutils.CreateFolder(ctx, outFolder)
	if err != nil {
		return err
	}
	tmpFolder := dir.Append("../tmp")
	err = wfutils.CreateFolder(ctx, tmpFolder)
	if err != nil {
		return err
	}
	errorFolder := dir.Append("../error")
	err = wfutils.CreateFolder(ctx, errorFolder)
	if err != nil {
		return err
	}
	processedFolder := dir.Append("../processed")
	err = wfutils.CreateFolder(ctx, processedFolder)
	if err != nil {
		return err
	}

	var transcodeOutput *activities.EncodeResult
	switch params.FolderName {
	case common.FolderProRes422HQHD:
		err = wfutils.Execute(ctx, activities.Video.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  tmpFolder,
			Resolution: "1920x1080",
			FrameRate:  25,
		}).Get(ctx, &transcodeOutput)
	case common.FolderProRes422HQNative:
		err = wfutils.Execute(ctx, activities.Video.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:  path,
			OutputDir: tmpFolder,
		}).Get(ctx, &transcodeOutput)
	case common.FolderProRes422HQNative25FPS:
		err = wfutils.Execute(ctx, activities.Video.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:  path,
			OutputDir: tmpFolder,
			FrameRate: 25,
		}).Get(ctx, &transcodeOutput)
	case common.FolderProRes4444K25FPS:
		err = wfutils.Execute(ctx, activities.Video.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  tmpFolder,
			Resolution: "3840x2160",
			FrameRate:  25,
		}).Get(ctx, &transcodeOutput)
	case common.FolderAVCIntra100HD:
		err = wfutils.Execute(ctx, activities.Video.TranscodeToH264Activity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  tmpFolder,
			Resolution: "1920x1080",
			FrameRate:  25,
			Bitrate:    "100M",
		}).Get(ctx, &transcodeOutput)
	case common.FolderXDCAMHD422:
		err = wfutils.Execute(ctx, activities.Video.TranscodeToXDCAMActivity, activities.EncodeParams{
			FilePath:   path,
			OutputDir:  tmpFolder,
			Resolution: "1920x1080",
			FrameRate:  25,
			Bitrate:    "60M",
		}).Get(ctx, &transcodeOutput)
	case common.FolderTranscribe:
		ctx = workflow.WithTaskQueue(ctx, environment.GetWorkerQueue())
		err = workflow.ExecuteChildWorkflow(ctx, TranscribeFile, TranscribeFileInput{
			Language:        "auto",
			File:            path.Linux(),
			DestinationPath: outFolder.Linux(),
		}).Get(ctx, nil)
	default:
		err = fmt.Errorf("codec not supported: %s", params.FolderName)
	}

	ctx = workflow.WithTaskQueue(ctx, environment.GetWorkerQueue())

	if err != nil {
		path, _ = wfutils.MoveToFolder(ctx, path, errorFolder, rclone.PriorityNormal)
		return err
	} else {
		path, _ = wfutils.MoveToFolder(ctx, path, processedFolder, rclone.PriorityNormal)

		if transcodeOutput != nil {
			_, _ = wfutils.MoveToFolder(ctx, transcodeOutput.OutputPath, outFolder, rclone.PriorityNormal)
		}
	}

	return nil
}
