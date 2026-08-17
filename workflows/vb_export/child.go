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

type transcodeFunc func(ctx workflow.Context, params VBExportChildWorkflowParams, outputDir paths.Path, isImage bool) (paths.Path, error)

// vbExportDestination describes one delivery destination. Exactly one of copySource
// and transcode must be set.
type vbExportDestination struct {
	flow      string
	folder    string
	outputDir string

	// ext is ignored when imageAware, which delivers an image under its own
	// extension and everything else as .mov.
	ext        string
	imageAware bool

	copySource func(VBExportChildWorkflowParams) paths.Path
	transcode  transcodeFunc
}

func runVBExportChild(ctx workflow.Context, params VBExportChildWorkflowParams, dest vbExportDestination) (*VBExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting VB export", "flow", dest.flow)

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

	rcloneDestination := deliveryFolder.Append(dest.folder, destName)

	err := wfutils.RcloneWaitForFileGone(ctx, rcloneDestination, telegram.ChatOslofjord, 10)
	if err != nil {
		return nil, err
	}

	if dest.transcode != nil {
		outputDir := params.TempDir.Append(dest.outputDir)
		err = wfutils.CreateFolder(ctx, outputDir)
		if err != nil {
			return nil, err
		}

		filePath, err = dest.transcode(ctx, params, outputDir, isImage)
		if err != nil {
			return nil, err
		}
	}

	err = wfutils.RcloneCopyFileWithNotifications(ctx, filePath, rcloneDestination, rclone.PriorityHigh, rcloneNotificationOptions)
	if err != nil {
		return nil, err
	}

	notifyExportDone(ctx, params, dest.flow, filePath)

	return &VBExportResult{
		ID:    params.ParentParams.VXID,
		Title: params.OriginalFilenameWithoutExt,
	}, nil
}

func transcodeToProRes(interlace, alpha bool) transcodeFunc {
	return func(ctx workflow.Context, params VBExportChildWorkflowParams, outputDir paths.Path, isImage bool) (paths.Path, error) {
		if isImage {
			return params.InputFile, nil
		}

		res, err := wfutils.Execute(ctx, activities.Video.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:       params.InputFile,
			OutputDir:      outputDir,
			Resolution:     utils.Resolution1080,
			FrameRate:      50,
			Interlace:      interlace,
			BurnInSubtitle: params.SubtitleFile,
			SubtitleStyle:  params.SubtitleStyle,
			Alpha:          alpha,
		}).Result(ctx)
		if err != nil {
			return paths.Path{}, err
		}

		return res.OutputPath, nil
	}
}
