package wfutils

import (
	"encoding/xml"
	"fmt"
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
	return Execute(ctx, activities.CreateFolder, activities.CreateFolderInput{
		Destination: destination,
	}).Get(ctx, nil)
}

func StandardizeFileName(ctx workflow.Context, file paths.Path) (paths.Path, error) {
	var result activities.FileResult
	err := Execute(ctx, activities.StandardizeFileName, activities.FileInput{
		Path: file,
	}).Get(ctx, &result)
	return result.Path, err
}

func MoveFile(ctx workflow.Context, source, destination paths.Path) error {
	external := source.OnExternalDrive() || destination.OnExternalDrive()

	if external {
		return Execute(ctx, activities.RcloneMoveFile, activities.RcloneFileInput{
			Source:      source,
			Destination: destination,
		}).Get(ctx, nil)
	} else {
		return Execute(ctx, activities.MoveFile, activities.MoveFileInput{
			Source:      source,
			Destination: destination,
		}).Get(ctx, nil)
	}
}

func CopyFile(ctx workflow.Context, source, destination paths.Path) error {
	return Execute(ctx, activities.CopyFile, activities.MoveFileInput{
		Source:      source,
		Destination: destination,
	}).Get(ctx, nil)
}

func MoveToFolder(ctx workflow.Context, file, folder paths.Path) (paths.Path, error) {
	newPath := folder.Append(file.Base())
	err := MoveFile(ctx, file, newPath)
	return newPath, err
}

func WriteFile(ctx workflow.Context, file paths.Path, data []byte) error {
	return Execute(ctx, activities.WriteFile, activities.WriteFileInput{
		Path: file,
		Data: data,
	}).Get(ctx, nil)
}

func ReadFile(ctx workflow.Context, file paths.Path) ([]byte, error) {
	var res []byte
	err := Execute(ctx, activities.ReadFile, activities.FileInput{
		Path: file,
	}).Get(ctx, &res)
	return res, err
}

func ListFiles(ctx workflow.Context, path paths.Path) (paths.Files, error) {
	var res []paths.Path
	err := Execute(ctx, activities.ListFiles, activities.FileInput{
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
	return Execute(ctx, activities.DeletePath, activities.DeletePathInput{
		Path: path,
	}).Get(ctx, nil)
}

func DeletePathRecursively(ctx workflow.Context, path paths.Path) error {
	return Execute(ctx, activities.DeletePath, activities.DeletePathInput{
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
	mimeType, err := Execute(ctx, activities.GetMimeType, activities.AnalyzeFileParams{
		FilePath: file,
	}).Result(ctx)
	if err != nil {
		return false, err
	}
	return strings.HasPrefix(*mimeType, "image"), nil
}
