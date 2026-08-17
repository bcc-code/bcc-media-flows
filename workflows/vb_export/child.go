package vb_export

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/bcc-code/bcc-media-flows/utils"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type transcodeFunc func(ctx workflow.Context, params VBExportChildWorkflowParams, outputDir paths.Path) (paths.Path, error)

// vbExportDestination describes one delivery destination. Exactly one of copySource
// and transcode must be set.
type vbExportDestination struct {
	destination Destination

	// ext is ignored when imageAware, which delivers an image under its own
	// extension and everything else as .mov.
	ext        string
	imageAware bool

	copySource func(VBExportChildWorkflowParams) paths.Path
	transcode  transcodeFunc

	// image runs in place of transcode when the input is an image. Nil delivers the
	// input untouched.
	image transcodeFunc
}

func runVBExportChild(ctx workflow.Context, params VBExportChildWorkflowParams, dest vbExportDestination) (*VBExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting VB export", "destination", dest.destination.Value)

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	var isImage bool
	if dest.imageAware {
		var err error
		isImage, err = wfutils.IsImage(ctx, params.InputFile)
		if err != nil {
			return nil, err
		}
	}

	filePath := params.InputFile
	destName := ""
	if dest.copySource != nil {
		filePath = dest.copySource(params)
		destName = filePath.Base()
	} else {
		ext := dest.ext
		if dest.imageAware {
			ext = ".mov"
			if isImage {
				ext = params.InputFile.Ext()
			}
		}

		extraFileName := ""
		if params.SubtitleFile != nil && !isImage {
			extraFileName = "_SUB_NOR"
		}

		destName = params.OriginalFilenameWithoutExt + extraFileName + ext
	}

	rcloneDestination := deliveryFolder.Append(dest.destination.DeliveryFolder(), destName)

	err := wfutils.RcloneWaitForFileGone(ctx, rcloneDestination, telegram.ChatOslofjord, 10)
	if err != nil {
		return nil, err
	}

	if dest.transcode != nil {
		outputDir := params.TempDir.Append(dest.destination.OutputDirName())
		err = wfutils.CreateFolder(ctx, outputDir)
		if err != nil {
			return nil, err
		}

		produce := dest.transcode
		if isImage {
			produce = dest.image
		}

		if produce != nil {
			filePath, err = produce(ctx, params, outputDir)
			if err != nil {
				return nil, err
			}
		}
	}

	err = wfutils.RcloneCopyFileWithNotifications(ctx, filePath, rcloneDestination, rclone.PriorityHigh, rcloneNotificationOptions)
	if err != nil {
		return nil, err
	}

	notifyExportDone(ctx, params, dest.destination, filePath)

	return &VBExportResult{
		ID:    params.ParentParams.VXID,
		Title: params.OriginalFilenameWithoutExt,
	}, nil
}

type proRes struct {
	interlace bool
	alpha     bool
}

func (p proRes) transcode(ctx workflow.Context, params VBExportChildWorkflowParams, outputDir paths.Path) (paths.Path, error) {
	res, err := wfutils.Execute(ctx, activities.Video.TranscodeToProResActivity, activities.EncodeParams{
		FilePath:       params.InputFile,
		OutputDir:      outputDir,
		Resolution:     utils.Resolution1080,
		FrameRate:      50,
		Interlace:      p.interlace,
		BurnInSubtitle: params.SubtitleFile,
		SubtitleStyle:  params.SubtitleStyle,
		Alpha:          p.alpha,
	}).Result(ctx)
	if err != nil {
		return paths.Path{}, err
	}

	return res.OutputPath, nil
}
