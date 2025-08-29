package miscworkflows

import (
	"encoding/json"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
	"strings"
	"time"
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

type MASVMetadata struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Package   Package   `json:"package"`
}
type Files struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name"`
	Path string `json:"path"`
	Size int    `json:"size"`
}
type Metadata struct {
	Church             string `json:"church"`
	PackageDescription string `json:"package_description"`
	PackageName        string `json:"package_name"`
	SenderEmail        string `json:"sender_email"`
}
type Package struct {
	ID         string    `json:"id"`
	Files      []Files   `json:"files"`
	Name       string    `json:"name"`
	PortalID   string    `json:"portal_id"`
	PortalName string    `json:"portal_name"`
	Sender     string    `json:"sender"`
	Size       int       `json:"size"`
	State      string    `json:"state"`
	TotalFiles int       `json:"total_files"`
	Metadata   Metadata  `json:"metadata"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func MASVImport(ctx workflow.Context, params MASVImportParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting MASVImport workflow", "id", params.ID, "name", params.Name)

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	outputDestination := paths.MustParse("/mnt/isilon/Input/FromMASV").Append(params.ID)

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

	srcFolder, err := paths.Parse(src)
	if err != nil {
		return err
	}

	var metaFileInfo *rclone.RcloneFile
	for i := 0; i < 60; i++ {

		files, err := wfutils.RcloneListFiles(ctx, srcFolder)
		if err != nil {
			return err
		}

		for _, file := range files {

			if !strings.HasSuffix(file.Name, "transfer-manifest.json") {
				continue
			}

			metaFileInfo = &file
			break
		}

		if metaFileInfo != nil {
			break
		}

		err = workflow.Sleep(ctx, 30*time.Second)
	}

	if metaFileInfo == nil {
		return fmt.Errorf("could not find metadata file for package %s", params.ID)
	}

	metaRemotePath, err := paths.Parse("s3prod:/" + metaFileInfo.Path)
	if err != nil {
		return err
	}

	metaFilePath := tmpRoot.Append("manifest.json")
	err = wfutils.CopyFile(ctx, metaRemotePath, metaFilePath)
	if err != nil {
		return err
	}

	metaBytes, err := wfutils.ReadFile(ctx, metaFilePath)
	if err != nil {
		return err
	}

	masvMeta := &MASVMetadata{}
	err = json.Unmarshal(metaBytes, masvMeta)
	if err != nil {
		return err
	}

	var transcodeJobs []wfutils.Task[*activities.EncodeResult]
	for _, f := range masvMeta.Package.Files {
		fpath := fmt.Sprintf("s3prod:/massiveio-bccm/%s/%s", f.Path, f.Name)
		parsedPath, err := paths.Parse(fpath)
		if err != nil {
			return err
		}

		err = wfutils.RcloneWaitForFileExists(ctx, parsedPath, 30)
		if err != nil {
			return err
		}

		tempFilePath := tmpRoot.Append(parsedPath.Base())
		err = wfutils.RcloneCopyFile(ctx, parsedPath, tempFilePath, rclone.PriorityNormal)
		if err != nil {
			return err
		}

		if lo.Contains([]string{".mov", ".avi", ".mxf", ".mp4"}, tempFilePath.Ext()) {
			// Transcode to ProRes
			job := wfutils.Execute(ctx, activities.Video.TranscodeToProResActivity, activities.EncodeParams{
				FilePath:  tempFilePath,
				OutputDir: outputDestination,
			})
			transcodeJobs = append(transcodeJobs, job)
			err = wfutils.RcloneCopyFile(ctx, tempFilePath, outputDestination.Append("originals").Append(tempFilePath.Base()), rclone.PriorityNormal)
		} else {
			err = wfutils.RcloneCopyFile(ctx, tempFilePath, outputDestination.Append(tempFilePath.Base()), rclone.PriorityNormal)
		}

		if err != nil {
			return err
		}
	}

	for _, j := range transcodeJobs {
		err := j.Wait(ctx)
		if err != nil {
			return err
		}
	}

	// Notify completion
	wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("✅ Copied MASV package '%s' to %s", params.Name, dst.Rclone()))

	return nil
}
