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
	"regexp"
	"strings"
	"time"
)

// Precompiled regex to sanitize church names
var churchSanitizer = regexp.MustCompile(`[^a-zA-Z0-9-]`)

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

	masvMeta, err := waitForTransferManifest(ctx, srcFolder, tmpRoot, params.ID)
	if err != nil {
		return err
	}

	filesToCopy, church, err := stagePackageFiles(ctx, masvMeta, tmpRoot)
	if err != nil {
		return err
	}

	outputDestination := paths.MustParse("/mnt/isilon/Input/FromMASV").Append(church).Append(params.ID)

	transcodeJobs, err := deliverPackageFiles(ctx, filesToCopy, outputDestination)
	if err != nil {
		return err
	}

	for _, job := range transcodeJobs {
		err = job.Wait(ctx)
		if err != nil {
			return err
		}
	}

	// Notify completion
	wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("✅ Copied MASV package '%s' to %s", params.Name, dst.Rclone()))

	return nil
}

// waitForTransferManifest polls the upload folder for up to half an hour: MASV reports
// the package finalized before the manifest has necessarily landed.
func waitForTransferManifest(ctx workflow.Context, srcFolder, tmpRoot paths.Path, packageID string) (*MASVMetadata, error) {
	var manifest *rclone.RcloneFile

	for i := 0; i < 60 && manifest == nil; i++ {
		files, err := wfutils.RcloneListFiles(ctx, srcFolder)
		if err != nil {
			return nil, err
		}

		for _, file := range files {
			if strings.HasSuffix(file.Name, "transfer-manifest.json") {
				manifest = &file
				break
			}
		}

		if manifest == nil {
			_ = workflow.Sleep(ctx, 30*time.Second)
		}
	}

	if manifest == nil {
		return nil, fmt.Errorf("could not find metadata file for package %s", packageID)
	}

	remotePath, err := paths.Parse("s3prod:/" + manifest.Path)
	if err != nil {
		return nil, err
	}

	localPath := tmpRoot.Append("manifest.json")
	err = wfutils.CopyFile(ctx, remotePath, localPath)
	if err != nil {
		return nil, err
	}

	contents, err := wfutils.ReadFile(ctx, localPath)
	if err != nil {
		return nil, err
	}

	masvMeta := &MASVMetadata{}
	err = json.Unmarshal(contents, masvMeta)
	if err != nil {
		return nil, err
	}

	return masvMeta, nil
}

// stagePackageFiles copies the package into temp storage and reads the church out of
// its metadata.json, which is what the delivery folder is named after.
func stagePackageFiles(ctx workflow.Context, masvMeta *MASVMetadata, tmpRoot paths.Path) ([]paths.Path, string, error) {
	church := "unknown"
	var staged []paths.Path

	for _, f := range masvMeta.Package.Files {
		remotePath, err := paths.Parse(fmt.Sprintf("s3prod:/massiveio-bccm/%s/%s", f.Path, f.Name))
		if err != nil {
			return nil, "", err
		}

		err = wfutils.RcloneWaitForFileExists(ctx, remotePath, 30)
		if err != nil {
			return nil, "", err
		}

		tempFilePath := tmpRoot.Append(remotePath.Base())
		err = wfutils.RcloneCopyFile(ctx, remotePath, tempFilePath, rclone.PriorityNormal)
		if err != nil {
			return nil, "", err
		}

		if strings.HasSuffix(tempFilePath.Base(), "metadata.json") {
			contents, err := wfutils.ReadFile(ctx, tempFilePath)
			if err != nil {
				return nil, "", err
			}

			metadata := &Metadata{}
			err = json.Unmarshal(contents, metadata)
			if err != nil {
				return nil, "", err
			}

			church = churchSanitizer.ReplaceAllString(metadata.Church, "_")
		}

		staged = append(staged, tempFilePath)
	}

	return staged, church, nil
}

// deliverPackageFiles copies each staged file to its destination, and for the video
// ones also starts a ProRes transcode, whose jobs the caller waits on.
func deliverPackageFiles(ctx workflow.Context, files []paths.Path, destination paths.Path) ([]wfutils.Task[*activities.EncodeResult], error) {
	var transcodeJobs []wfutils.Task[*activities.EncodeResult]

	for _, file := range files {
		var err error

		if lo.Contains([]string{".mov", ".avi", ".mxf", ".mp4"}, file.Ext()) {
			transcodeJobs = append(transcodeJobs, wfutils.Execute(ctx, activities.Video.TranscodeToProResActivity, activities.EncodeParams{
				FilePath:  file,
				OutputDir: destination,
			}))

			err = wfutils.RcloneCopyFile(ctx, file, destination.Append("originals").Append(file.Base()), rclone.PriorityNormal)
		} else {
			err = wfutils.RcloneCopyFile(ctx, file, destination.Append(file.Base()), rclone.PriorityNormal)
		}

		if err != nil {
			return nil, err
		}
	}

	return transcodeJobs, nil
}
