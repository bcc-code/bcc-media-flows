package export

import (
	"path/filepath"
	"strings"

	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"github.com/bcc-code/bccm-flows/workflows"
	"go.temporal.io/sdk/workflow"
)

func VXExportToPlayout(ctx workflow.Context, params VXExportChildWorklowParams) (*VXExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportToPlayout")

	options := workflows.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	xdcamOutputDir := filepath.Join(params.TempDir, "xdcam_output")
	err := wfutils.CreateFolder(ctx, xdcamOutputDir)
	if err != nil {
		return nil, err
	}

	options.TaskQueue = utils.GetTranscodeQueue()
	ctx = workflow.WithActivityOptions(ctx, options)

	// Transcode video using playout encoding
	var videoResult common.VideoResult
	err = workflow.ExecuteActivity(ctx, activities.TranscodeToXDCAMActivity, activities.EncodeParams{
		Bitrate:    "50M",
		FilePath:   params.MergeResult.VideoFile,
		OutputDir:  xdcamOutputDir,
		Resolution: r1080p,
		FrameRate:  25,
	}).Get(ctx, &videoResult)
	if err != nil {
		return nil, err
	}

	// Mux into MXF file with 16 audio channels
	var muxResult *common.PlayoutMuxResult
	err = workflow.ExecuteActivity(ctx, activities.TranscodePlayoutMux, common.PlayoutMuxInput{
		VideoFilePath:     videoResult.OutputPath,
		AudioFilePaths:    params.MergeResult.AudioFiles,
		SubtitleFilePaths: params.MergeResult.SubtitleFiles,
		OutputDir:         params.OutputDir,
		FallbackLanguage:  "nor",
	}).Get(ctx, &muxResult)
	if err != nil {
		return nil, err
	}

	options.TaskQueue = utils.GetWorkerQueue()
	ctx = workflow.WithActivityOptions(ctx, options)

	// Rclone to playout
	source := strings.Replace(muxResult.Path, utils.GetIsilonPrefix()+"/", "isilon:isilon/", 1)
	destination := "playout:/dropbox" + filepath.Base(muxResult.Path)
	err = workflow.ExecuteActivity(ctx, activities.RcloneCopy, activities.RcloneCopyInput{
		Source:      source,
		Destination: destination,
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &VXExportResult{
		ID:       params.ParentParams.VXID,
		Title:    params.ExportData.Title,
		Duration: formatSecondsToTimestamp(params.MergeResult.Duration),
	}, nil
}
