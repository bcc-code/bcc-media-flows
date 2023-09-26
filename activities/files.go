package activities

import (
	"context"
	"github.com/bcc-code/bccm-flows/utils"
	"go.temporal.io/sdk/activity"
	"os"
	"path/filepath"
)

type FileInput struct {
	Path string
}

type FileResult struct {
	Path string
}

type MoveFileInput struct {
	Source      string
	Destination string
}

func MoveFile(ctx context.Context, input MoveFileInput) (*FileResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "MoveFile")
	log.Info("Starting MoveFileActivity")

	err := os.MkdirAll(filepath.Dir(input.Destination), os.ModePerm)
	if err != nil {
		return nil, err
	}
	err = os.Rename(input.Source, input.Destination)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(input.Destination, os.ModePerm)
	return &FileResult{
		Path: input.Destination,
	}, nil
}

func StandardizeFileName(ctx context.Context, input FileInput) (*FileResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "StandardizeFileName")
	log.Info("Starting StandardizeFileNameActivity")

	path := utils.FixFilename(input.Path)
	err := os.Rename(input.Path, path)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(path, os.ModePerm)
	return &FileResult{
		Path: path,
	}, nil
}

type CreateFolderInput struct {
	Destination string
}

func CreateFolder(ctx context.Context, input CreateFolderInput) error {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "CreateFolder")
	log.Info("Starting CreateFolderActivity")

	err := os.MkdirAll(input.Destination, os.ModePerm)
	if err != nil {
		return err
	}
	return os.Chmod(input.Destination, os.ModePerm)
}

type WriteFileInput struct {
	Path string
	Data []byte
}

func WriteFile(ctx context.Context, input WriteFileInput) error {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "WriteFile")
	log.Info("Starting WriteFileActivity")

	err := os.MkdirAll(filepath.Dir(input.Path), os.ModePerm)
	if err != nil {
		return err
	}
	err = os.WriteFile(input.Path, input.Data, os.ModePerm)
	if err != nil {
		return err
	}
	_ = os.Chmod(input.Path, os.ModePerm)
	return nil
}

func ReadFile(ctx context.Context, input FileInput) ([]byte, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "ReadFile")
	log.Info("Starting ReadFileActivity")

	return os.ReadFile(input.Path)
}

func DeletePath(ctx context.Context, input FileInput) error {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "DeletePath")
	log.Info("Starting DeletePathActivity")

	return os.RemoveAll(input.Path)
}
