package miscworkflows

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type FixDurationVXInput struct {
	VXID string
}

func FixDurationVX(
	ctx workflow.Context,
	params FixDurationVXInput,
) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting FixDurationVX")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	// Get the original file from Vidispine
	originalFile, err := wfutils.Execute(ctx, activities.Vidispine.GetFileFromVXActivity, vsactivity.GetFileFromVXParams{
		VXID: params.VXID,
		Tags: []string{"original"},
	}).Result(ctx)
	if err != nil {
		return fmt.Errorf("failed to get original file: %w", err)
	}

	// Get temp folder for processing
	tempFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return fmt.Errorf("failed to get temp folder: %w", err)
	}

	// Create output path in temp folder
	outputPath := tempFolder.Append(originalFile.FilePath.Base()).SetExt(originalFile.FilePath.Ext())

	// Run FFmpeg to fix duration
	_, err = wfutils.Execute(ctx, activities.Video.FixDurationActivity, activities.FixDurationInput{
		InputPath:  originalFile.FilePath,
		OutputPath: outputPath,
	}).Result(ctx)
	if err != nil {
		return fmt.Errorf("failed to fix duration: %w", err)
	}

	// Replace the original file on disk with the fixed version
	err = wfutils.MoveFile(ctx, outputPath, originalFile.FilePath, rclone.PriorityNormal)
	if err != nil {
		return fmt.Errorf("failed to replace original file: %w", err)
	}

	wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("✅ Duration fix completed for %s", params.VXID))

	return nil
}
