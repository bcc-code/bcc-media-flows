package wfutils

import (
	"encoding/xml"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"go.temporal.io/sdk/temporal"
	"path/filepath"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
)

func CreateFolder(ctx workflow.Context, destination paths.Path) error {
	return Execute(ctx, activities.Util.CreateFolder, activities.CreateFolderInput{
		Destination: destination,
	}).Get(ctx, nil)
}

func StandardizeFileName(ctx workflow.Context, file paths.Path) (paths.Path, error) {
	var result activities.FileResult
	err := Execute(ctx, activities.Util.StandardizeFileName, activities.FileInput{
		Path: file,
	}).Get(ctx, &result)
	return result.Path, err
}

func MoveFile(ctx workflow.Context, source, destination paths.Path, priority rclone.Priority) error {
	external := source.OnExternalDrive() || destination.OnExternalDrive()

	if external {
		return RcloneMoveFile(ctx, source, destination, priority)
	} else {
		return Execute(ctx, activities.Util.MoveFile, activities.MoveFileInput{
			Source:      source,
			Destination: destination,
		}).Get(ctx, nil)
	}
}

func CopyFile(ctx workflow.Context, source, destination paths.Path) error {
	return Execute(ctx, activities.Util.CopyFile, activities.MoveFileInput{
		Source:      source,
		Destination: destination,
	}).Get(ctx, nil)
}

func CopyToFolder(ctx workflow.Context, file, folder paths.Path, priority rclone.Priority) (paths.Path, error) {
	newPath := folder.Append(file.Base())
	err := CopyFile(ctx, file, newPath)
	return newPath, err
}

func MoveToFolder(ctx workflow.Context, file, folder paths.Path, priority rclone.Priority) (paths.Path, error) {
	newPath := folder.Append(file.Base())
	err := MoveFile(ctx, file, newPath, priority)
	return newPath, err
}

func WriteFile(ctx workflow.Context, file paths.Path, data []byte) error {
	return Execute(ctx, activities.Util.WriteFile, activities.WriteFileInput{
		Path: file,
		Data: data,
	}).Get(ctx, nil)
}

func ReadFile(ctx workflow.Context, file paths.Path) ([]byte, error) {
	var res []byte
	err := Execute(ctx, activities.Util.ReadFile, activities.FileInput{
		Path: file,
	}).Get(ctx, &res)
	return res, err
}

func ListFiles(ctx workflow.Context, path paths.Path) (paths.Files, error) {
	var res []paths.Path
	err := Execute(ctx, activities.Util.ListFiles, activities.FileInput{
		Path: path,
	}).Get(ctx, &res)
	return res, err
}

func UnmarshalXMLFile[T any](ctx workflow.Context, file paths.Path) (*T, error) {
	var r T
	res, err := ReadFile(ctx, file)
	if err != nil {
		return nil, err
	}
	err = xml.Unmarshal(res, &r)
	return &r, err
}

func DeletePath(ctx workflow.Context, path paths.Path) error {
	return Execute(ctx, activities.Util.DeletePath, activities.DeletePathInput{
		Path: path,
	}).Get(ctx, nil)
}

func DeletePathRecursively(ctx workflow.Context, path paths.Path) error {
	return Execute(ctx, activities.Util.DeletePath, activities.DeletePathInput{
		RemoveAll: true,
		Path:      path,
	}).Get(ctx, nil)
}

func GetWorkflowLucidLinkOutputFolder(ctx workflow.Context, root string) paths.Path {
	info := workflow.GetInfo(ctx)

	date := time.Now()

	path := paths.New(
		paths.LucidLinkDrive,
		filepath.Join(
			root,
			fmt.Sprintf("%d/%d/%d", date.Year(), date.Month(), date.Day()),
			info.OriginalRunID,
		),
	)

	return path
}

func GetWorkflowIsilonOutputFolder(ctx workflow.Context, root string) (paths.Path, error) {
	info := workflow.GetInfo(ctx)

	date := time.Now()

	path := paths.New(
		paths.IsilonDrive,
		filepath.Join(root, fmt.Sprintf("%d/%d/%d", date.Year(), date.Month(), date.Day()), info.OriginalRunID),
	)

	return path, CreateFolder(ctx, path)
}

func GetWorkflowMastersOutputFolder(ctx workflow.Context) (paths.Path, error) {
	return GetWorkflowIsilonOutputFolder(ctx, "Production/masters")
}

func GetWorkflowRawOutputFolder(ctx workflow.Context) (paths.Path, error) {
	return GetWorkflowIsilonOutputFolder(ctx, "Production/raw")
}

func GetWorkflowAuxOutputFolder(ctx workflow.Context) (paths.Path, error) {
	return GetWorkflowIsilonOutputFolder(ctx, "Production/aux")
}

func GetWorkflowTempFolder(ctx workflow.Context) (paths.Path, error) {
	info := workflow.GetInfo(ctx)

	path := paths.MustParse(filepath.Join(environment.GetTempMountPrefix(), "workflows", info.OriginalRunID))

	return path, CreateFolder(ctx, path)
}

// GetMapKeysSafely makes sure that the order of the keys returned are identical to other workflow executions.
func GetMapKeysSafely[K comparable, T any](ctx workflow.Context, m map[K]T) ([]K, error) {
	var keys []K
	err := workflow.SideEffect(ctx, func(ctx workflow.Context) any {
		return lo.Keys(m)
	}).Get(&keys)
	return keys, err
}

func IsImage(ctx workflow.Context, file paths.Path) (bool, error) {
	mimeType, err := Execute(ctx, activities.Util.GetMimeType, activities.AnalyzeFileParams{
		FilePath: file,
	}).Result(ctx)
	if err != nil {
		return false, err
	}
	return strings.HasPrefix(*mimeType, "image"), nil
}

func RcloneCheckFileExists(ctx workflow.Context, file paths.Path) (bool, error) {
	return Execute(ctx, activities.Util.RcloneCheckFileExists, activities.RcloneSingleFileInput{
		File: file,
	}).Result(ctx)
}

func RcloneWaitForFileGone(ctx workflow.Context, file paths.Path, notificationChannel telegram.Chat, retries int) error {
	fileExists := true

	template := &notifications.Simple{}
	msg, _ := telegram.NewMessage(notificationChannel, template)

	for i := 0; i < retries; i++ {
		exists, err := RcloneCheckFileExists(ctx, file)
		if err != nil {
			return err
		}

		if !exists {
			// The file does not exist, so we can continue with upload
			fileExists = false
			break
		}

		template.Message = fmt.Sprintf("⚠️ File ```%s``` still exists, retrying in one minute (%d/%d)", file.Rclone(), i+1, retries)
		msg.UpdateWithTemplate(template)
		msg = SendTelegramMessage(ctx, notificationChannel, msg)

		workflow.Sleep(ctx, time.Minute)
	}

	if fileExists {
		return temporal.NewNonRetryableApplicationError("File already exists", "FILE_EXISTS", nil)
	}

	return nil
}

func RcloneCopyFile(ctx workflow.Context, source, destination paths.Path, priority rclone.Priority) error {
	return RcloneCopyFileWithNotifications(ctx, source, destination, priority, nil)
}

func RcloneCopyFileWithNotifications(ctx workflow.Context, source, destination paths.Path, priority rclone.Priority, notificationOptions *activities.TelegramNotificationOptions) error {
	jobID, err := Execute(ctx, activities.Util.RcloneCopyFile, activities.RcloneFileInput{
		Source:      source,
		Destination: destination,
		Priority:    priority,
	}).Result(ctx)
	if err != nil {
		return err
	}
	success, err := Execute(ctx, activities.Util.RcloneWaitForJob, activities.RcloneWaitForJobInput{
		JobID:               jobID,
		NotificationOptions: notificationOptions,
	}).Result(ctx)
	if err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("rclone job failed")
	}
	return nil
}

func RcloneMoveFile(ctx workflow.Context, source, destination paths.Path, priority rclone.Priority) error {
	jobID, err := Execute(ctx, activities.Util.RcloneMoveFile, activities.RcloneFileInput{
		Source:      source,
		Destination: destination,
		Priority:    priority,
	}).Result(ctx)
	if err != nil {
		return err
	}
	success, err := Execute(ctx, activities.Util.RcloneWaitForJob, activities.RcloneWaitForJobInput{JobID: jobID}).Result(ctx)
	if err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("rclone job failed")
	}
	return nil
}

func RcloneCopyDir(ctx workflow.Context, source, destination string, priority rclone.Priority) error {
	jobID, err := Execute(ctx, activities.Util.RcloneCopyDir, activities.RcloneCopyDirInput{
		Source:      source,
		Destination: destination,
		Priority:    priority,
	}).Result(ctx)
	if err != nil {
		return err
	}
	success, err := Execute(ctx, activities.Util.RcloneWaitForJob, activities.RcloneWaitForJobInput{
		JobID: jobID,
	}).Result(ctx)
	if err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("rclone job failed")
	}
	return nil
}
