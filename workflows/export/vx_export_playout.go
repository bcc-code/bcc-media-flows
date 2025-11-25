package export

import (
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/bcc-code/bcc-media-flows/utils"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

func VXExportToXDCAM(ctx workflow.Context, params VXExportChildWorkflowParams) (*VXExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportToXDCAM")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	xdcamOutputDir := params.TempDir.Append("xdcam_output")
	err := wfutils.CreateFolder(ctx, xdcamOutputDir)
	if err != nil {
		return nil, err
	}

	// Transcode video using playout encoding
	var videoResult common.VideoResult
	err = wfutils.Execute(ctx, activities.Video.TranscodeToXDCAMActivity, activities.EncodeParams{
		Bitrate:    "50M",
		FilePath:   *params.MergeResult.VideoFile,
		OutputDir:  xdcamOutputDir,
		Resolution: utils.Resolution1080,
		FrameRate:  25,
		Interlace:  true,
	}).Get(ctx, &videoResult)
	if err != nil {
		return nil, err
	}

	// Mux into MXF file with 16 audio channels
	var muxResult *common.PlayoutMuxResult
	err = wfutils.Execute(ctx, activities.Video.TranscodePlayoutMux, common.PlayoutMuxInput{
		VideoFilePath:     videoResult.OutputPath,
		AudioFilePaths:    params.MergeResult.AudioFiles,
		SubtitleFilePaths: params.MergeResult.SubtitleFiles,
		OutputDir:         params.OutputDir,
		FallbackLanguage:  "nor",
	}).Get(ctx, &muxResult)
	if err != nil {
		return nil, err
	}

	destination := "brunstad:/Delivery/XDCAM"
	err = wfutils.RcloneCopyDir(ctx, params.OutputDir.Rclone(), destination, rclone.PriorityNormal)
	if err != nil {
		return nil, err
	}

	notifyExportDone(ctx, telegram.ChatOslofjord, params, "xdcam", '🟩')

	return &VXExportResult{
		ID:       params.ParentParams.VXID,
		Title:    params.ExportData.SafeTitle,
		Duration: formatSecondsToTimestamp(params.MergeResult.Duration),
	}, nil
}
