package wfutils

import (
	"encoding/xml"
	"fmt"
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/environment"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
	"path/filepath"
	"time"
)

func CreateFolder(ctx workflow.Context, destination paths.Path) error {
	return workflow.ExecuteActivity(ctx, activities.CreateFolder, activities.CreateFolderInput{
		Destination: destination,
	}).Get(ctx, nil)
}

func StandardizeFileName(ctx workflow.Context, file paths.Path) (paths.Path, error) {
	var result activities.FileResult
	err := workflow.ExecuteActivity(ctx, activities.StandardizeFileName, activities.FileInput{
		Path: file,
	}).Get(ctx, &result)
	return result.Path, err
}

func MoveFile(ctx workflow.Context, source, destination paths.Path) error {
	return workflow.ExecuteActivity(ctx, activities.MoveFile, activities.MoveFileInput{
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
	return workflow.ExecuteActivity(ctx, activities.WriteFile, activities.WriteFileInput{
		Path: file,
		Data: data,
	}).Get(ctx, nil)
}

func ReadFile(ctx workflow.Context, file paths.Path) ([]byte, error) {
	var res []byte
	err := workflow.ExecuteActivity(ctx, activities.ReadFile, activities.FileInput{
		Path: file,
	}).Get(ctx, &res)
	return res, err
}

func ListFiles(ctx workflow.Context, path paths.Path) ([]paths.Path, error) {
	var res []paths.Path
	err := workflow.ExecuteActivity(ctx, activities.ListFiles, activities.FileInput{
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
	return workflow.ExecuteActivity(ctx, activities.DeletePath, activities.FileInput{
		Path: path,
	}).Get(ctx, nil)
}

func GetWorkflowIsilonOutputFolder(ctx workflow.Context, root string) (paths.Path, error) {
	info := workflow.GetInfo(ctx)

	date := time.Now()

	path := filepath.Join(environment.GetIsilonPrefix(), "Production", root, fmt.Sprintf("%d/%d/%d", date.Year(), date.Month(), date.Day()), info.OriginalRunID)

	return paths.MustParse(path), CreateFolder(ctx, paths.MustParse(path))
}

func GetWorkflowMastersOutputFolder(ctx workflow.Context) (paths.Path, error) {
	return GetWorkflowIsilonOutputFolder(ctx, "masters")
}

func GetWorkflowRawOutputFolder(ctx workflow.Context) (paths.Path, error) {
	return GetWorkflowIsilonOutputFolder(ctx, "raw")
}

func GetWorkflowAuxOutputFolder(ctx workflow.Context) (paths.Path, error) {
	return GetWorkflowIsilonOutputFolder(ctx, "aux")
}

func GetWorkflowTempFolder(ctx workflow.Context) (paths.Path, error) {
	info := workflow.GetInfo(ctx)

	path := filepath.Join(environment.GetTempMountPrefix(), "workflows", info.OriginalRunID)

	return paths.MustParse(path), CreateFolder(ctx, paths.MustParse(path))
}

// GetMapKeysSafely makes sure that the order of the keys returned are identical to other workflow executions.
func GetMapKeysSafely[T any](ctx workflow.Context, m map[string]T) ([]string, error) {
	var keys []string
	err := workflow.SideEffect(ctx, func(ctx workflow.Context) any {
		return lo.Keys(m)
	}).Get(&keys)
	return keys, err
}
