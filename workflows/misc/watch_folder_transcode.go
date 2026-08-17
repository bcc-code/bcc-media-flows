package miscworkflows

import (
	"context"
	"fmt"

	"github.com/bcc-code/bcc-media-flows/utils"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/transcode"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type WatchFolderTranscodeInput struct {
	Path       string
	FolderName string
}

// watchFolderEncodes are the folders whose transcode is one activity call.
// FilePath and OutputDir are filled in per run.
var watchFolderEncodes = map[string]struct {
	activity func(context.Context, activities.EncodeParams) (*activities.EncodeResult, error)
	params   activities.EncodeParams
}{
	common.FolderProRes422HQHD: {
		activities.Video.TranscodeToProResActivity,
		activities.EncodeParams{Resolution: utils.Resolution1080, FrameRate: 25},
	},
	common.FolderProRes422HQNative: {
		activities.Video.TranscodeToProResActivity,
		activities.EncodeParams{},
	},
	common.FolderProRes422HQNative25FPS: {
		activities.Video.TranscodeToProResActivity,
		activities.EncodeParams{FrameRate: 25},
	},
	common.FolderProRes4444K25FPS: {
		activities.Video.TranscodeToProResActivity,
		activities.EncodeParams{Resolution: utils.Resolution4K, FrameRate: 25},
	},
	common.FolderAVCIntra100HD: {
		activities.Video.TranscodeToAVCIntraActivity,
		activities.EncodeParams{Resolution: utils.Resolution1080, FrameRate: 25, Bitrate: "100M"},
	},
	common.FolderXDCAMHD422: {
		activities.Video.TranscodeToXDCAMActivity,
		activities.EncodeParams{Resolution: utils.Resolution1080, FrameRate: 25, Bitrate: "60M"},
	},
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
	if encode, ok := watchFolderEncodes[params.FolderName]; ok {
		encode.params.FilePath = path
		encode.params.OutputDir = tmpFolder
		err = wfutils.Execute(ctx, encode.activity, encode.params).Get(ctx, &transcodeOutput)
	} else {
		switch params.FolderName {
		case common.FolderTranscribe:
			ctx = workflow.WithTaskQueue(ctx, environment.GetWorkerQueue())
			ctx = wfutils.WithChildSearchAttributes(ctx, "")
			err = workflow.ExecuteChildWorkflow(ctx, TranscribeFile, TranscribeFileInput{
				Language:        "auto",
				File:            path.Linux(),
				DestinationPath: outFolder.Linux(),
			}).Get(ctx, nil)
		case common.FolderHAP50FPS:
			var hapResult *activities.HAPResult
			hapResult, err = wfutils.Execute(ctx, activities.Video.TranscodeToHAPActivity, activities.HAPInput{
				FilePath:  path,
				OutputDir: tmpFolder,
				Format:    transcode.HAPFormatHAPQ,
			}).Result(ctx)
			if err == nil && hapResult != nil {
				transcodeOutput = &activities.EncodeResult{
					OutputPath: hapResult.OutputPath,
				}
			}
		default:
			err = fmt.Errorf("codec not supported: %s", params.FolderName)
		}
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
