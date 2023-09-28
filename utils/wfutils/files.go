package wfutils

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
	"path/filepath"
	"time"
)

func CreateFolder(ctx workflow.Context, destination string) error {
	return workflow.ExecuteActivity(ctx, activities.CreateFolder, activities.CreateFolderInput{
		Destination: destination,
	}).Get(ctx, nil)
}

func StandardizeFileName(ctx workflow.Context, file string) (string, error) {
	var result activities.FileResult
	err := workflow.ExecuteActivity(ctx, activities.StandardizeFileName, activities.FileInput{
		Path: file,
	}).Get(ctx, &result)
	return result.Path, err
}

func MoveFile(ctx workflow.Context, source, destination string) error {
	return workflow.ExecuteActivity(ctx, activities.MoveFile, activities.MoveFileInput{
		Source:      source,
		Destination: destination,
	}).Get(ctx, nil)
}

func MoveToFolder(ctx workflow.Context, file, folder string) (string, error) {
	newPath := filepath.Join(folder, filepath.Base(file))
	err := MoveFile(ctx, file, newPath)
	return newPath, err
}

func WriteFile(ctx workflow.Context, file string, data []byte) error {
	return workflow.ExecuteActivity(ctx, activities.WriteFile, activities.WriteFileInput{
		Path: file,
		Data: data,
	}).Get(ctx, nil)
}

func DeletePath(ctx workflow.Context, path string) error {
	return workflow.ExecuteActivity(ctx, activities.DeletePath, activities.FileInput{
		Path: path,
	}).Get(ctx, nil)
}

func GetWorkflowOutputFolder(ctx workflow.Context) (string, error) {
	info := workflow.GetInfo(ctx)

	date := time.Now()

	path := fmt.Sprintf("%s/%04d/%02d/%02d/%s", utils.GetIsilonPrefix()+"/Production/aux", date.Year(), date.Month(), date.Day(), info.OriginalRunID)

	return path, CreateFolder(ctx, path)
}

func GetWorkflowTempFolder(ctx workflow.Context) (string, error) {
	info := workflow.GetInfo(ctx)

	path := fmt.Sprintf("%s/workflows/%s", utils.GetIsilonPrefix()+"/system/tmp", info.OriginalRunID)

	return path, CreateFolder(ctx, path)
}

// GetMapKeysSafely makes sure that the order of the keys returned are identical to other workflow executions.
func GetMapKeysSafely[T any](ctx workflow.Context, m map[string]T) ([]string, error) {
	var keys []string
	err := workflow.SideEffect(ctx, func(ctx workflow.Context) any {
		return lo.Keys(m)
	}).Get(&keys)
	return keys, err
}
