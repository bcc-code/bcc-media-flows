package workflows

import (
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/utils"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"path/filepath"
	"time"
)

func GetDefaultActivityOptions() workflow.ActivityOptions {
	return workflow.ActivityOptions{
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Minute * 1,
			MaximumAttempts: 10,
			MaximumInterval: time.Hour * 1,
		},
		StartToCloseTimeout:    time.Hour * 4,
		ScheduleToCloseTimeout: time.Hour * 48,
		HeartbeatTimeout:       time.Minute * 1,
		TaskQueue:              utils.GetWorkerQueue(),
	}
}

func GetDefaultWorkflowOptions() workflow.ChildWorkflowOptions {
	return workflow.ChildWorkflowOptions{
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Minute * 1,
			MaximumAttempts: 10,
			MaximumInterval: time.Hour * 1,
		},
		TaskQueue: utils.GetWorkerQueue(),
	}
}

func createFolder(ctx workflow.Context, destination string) error {
	return workflow.ExecuteActivity(ctx, activities.CreateFolder, activities.CreateFolderInput{
		Destination: destination,
	}).Get(ctx, nil)
}

func standardizeFileName(ctx workflow.Context, file string) (string, error) {
	var result activities.FileResult
	err := workflow.ExecuteActivity(ctx, activities.StandardizeFileName, activities.FileInput{
		Path: file,
	}).Get(ctx, &result)
	return result.Path, err
}

func moveFile(ctx workflow.Context, source, destination string) error {
	return workflow.ExecuteActivity(ctx, activities.MoveFile, activities.MoveFileInput{
		Source:      source,
		Destination: destination,
	}).Get(ctx, nil)
}

func moveToFolder(ctx workflow.Context, file, folder string) (string, error) {
	newPath := filepath.Join(filepath.Dir(folder), filepath.Base(file))
	err := moveFile(ctx, file, newPath)
	return newPath, err
}

func writeFile(ctx workflow.Context, file string, data []byte) error {
	return workflow.ExecuteActivity(ctx, activities.WriteFile, activities.WriteFileInput{
		Path: file,
		Data: data,
	}).Get(ctx, nil)
}

func deletePath(ctx workflow.Context, path string) error {
	return workflow.ExecuteActivity(ctx, activities.DeletePath, activities.FileInput{
		Path: path,
	}).Get(ctx, nil)
}

func getWorkflowOutputFolder(ctx workflow.Context) (string, error) {
	path := utils.GetWorkflowOutputFolder(ctx)
	return path, createFolder(ctx, path)
}

func getWorkflowTempFolder(ctx workflow.Context) (string, error) {
	path := utils.GetWorkflowTempFolder(ctx)
	return path, createFolder(ctx, path)
}
