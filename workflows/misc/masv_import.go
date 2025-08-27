package miscworkflows

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

// MASVImportParams contains the info received from Massive.app webhook
type MASVImportParams struct {
	ID         string
	Name       string
	Sender     string
	TotalFiles int
	EventID    string
	EventTime  string
}

func MASVImport(ctx workflow.Context, params MASVImportParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting MASVImport workflow", "id", params.ID, "name", params.Name)

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	message := fmt.Sprintf("📦 MASV package finalized\nID: %s\nName: %s\nSender: %s\nFiles: %d\nEvent: %s @ %s",
		params.ID, params.Name, params.Sender, params.TotalFiles, params.EventID, params.EventTime)

	wfutils.SendTelegramText(ctx, telegram.ChatOther, message)

	// Build source path for the package on s3prod and copy its contents to a workflow temp folder
	src := fmt.Sprintf("s3prod:/massiveio-bccm/upload/%s", params.Name)

	// Get workflow-specific temp folder and create a subfolder for this package
	tmpRoot, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return err
	}
	dst := tmpRoot.Append("masv", params.ID)
	if err := wfutils.CreateFolder(ctx, dst); err != nil {
		return err
	}

	// Copy the directory contents from s3 to temp using rclone
	if err := wfutils.RcloneCopyDir(ctx, src, dst.Rclone(), rclone.PriorityNormal); err != nil {
		return err
	}

	// Notify completion
	wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("✅ Copied MASV package '%s' to %s", params.Name, dst.Rclone()))

	return nil
}
