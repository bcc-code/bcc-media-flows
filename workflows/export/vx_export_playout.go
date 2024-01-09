package export

import (
	"path/filepath"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/environment"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

func VXExportToPlayout(ctx workflow.Context, params VXExportChildWorkflowParams) (*VXExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportToPlayout")

	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	xdcamOutputDir := params.TempDir.Append("xdcam_output")
	err := wfutils.CreateFolder(ctx, xdcamOutputDir)
	if err != nil {
		return nil, err
	}

	options.TaskQueue = environment.GetTranscodeQueue()
	ctx = workflow.WithActivityOptions(ctx, options)

	// Transcode video using playout encoding
	var videoResult common.VideoResult
	err = wfutils.ExecuteWithQueue(ctx, activities.TranscodeToXDCAMActivity, activities.EncodeParams{
		Bitrate:    "50M",
		FilePath:   *params.MergeResult.VideoFile,
		OutputDir:  xdcamOutputDir,
		Resolution: string(r1080p),
		FrameRate:  25,
		Interlace:  true,
	}).Get(ctx, &videoResult)
	if err != nil {
		return nil, err
	}

	// Mux into MXF file with 16 audio channels
	var muxResult *common.PlayoutMuxResult
	err = wfutils.ExecuteWithQueue(ctx, activities.TranscodePlayoutMux, common.PlayoutMuxInput{
		VideoFilePath:     videoResult.OutputPath,
		AudioFilePaths:    params.MergeResult.AudioFiles,
		SubtitleFilePaths: params.MergeResult.SubtitleFiles,
		OutputDir:         params.OutputDir,
		FallbackLanguage:  "nor",
	}).Get(ctx, &muxResult)
	if err != nil {
		return nil, err
	}

	options.TaskQueue = environment.GetWorkerQueue()
	ctx = workflow.WithActivityOptions(ctx, options)

	// Rclone to playout
	destination := "playout:/tmp"
	err = wfutils.ExecuteWithQueue(ctx, activities.RcloneCopyDir, activities.RcloneCopyDirInput{
		Source:      params.OutputDir.Rclone(),
		Destination: destination,
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	err = wfutils.ExecuteWithQueue(ctx, activities.FtpPlayoutRename, activities.FtpPlayoutRenameParams{
		From: filepath.Join("/tmp/", muxResult.Path.Base()),
		To:   filepath.Join("/dropbox/", muxResult.Path.Base()),
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	notifyExportDone(ctx, params, "playout")

	return &VXExportResult{
		ID:       params.ParentParams.VXID,
		Title:    params.ExportData.SafeTitle,
		Duration: formatSecondsToTimestamp(params.MergeResult.Duration),
	}, nil
}
