package activities

import (
	"context"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/samber/lo"
	"go.temporal.io/sdk/activity"
	"os"
	"path/filepath"
)

type FileInput struct {
	Path paths.Path
}

type FileResult struct {
	Path paths.Path
}

type MoveFileInput struct {
	Source      paths.Path
	Destination paths.Path
}

func MoveFile(ctx context.Context, input MoveFileInput) (*FileResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "MoveFile")
	log.Info("Starting MoveFileActivity")

	err := os.MkdirAll(filepath.Dir(input.Destination.Local()), os.ModePerm)
	if err != nil {
		return nil, err
	}
	err = os.Rename(input.Source.Local(), input.Destination.Local())
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(input.Destination.Local(), os.ModePerm)
	return &FileResult{
		Path: input.Destination,
	}, nil
}

func StandardizeFileName(ctx context.Context, input FileInput) (*FileResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "StandardizeFileName")
	log.Info("Starting StandardizeFileNameActivity")

	path := paths.FixFilename(input.Path.Local())
	err := os.Rename(input.Path.Local(), path)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(path, os.ModePerm)
	return &FileResult{
		Path: paths.MustParse(path),
	}, nil
}

type CreateFolderInput struct {
	Destination paths.Path
}

func CreateFolder(ctx context.Context, input CreateFolderInput) error {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "CreateFolder")
	log.Info("Starting CreateFolderActivity")

	err := os.MkdirAll(input.Destination.Local(), os.ModePerm)
	if err != nil {
		return err
	}
	return os.Chmod(input.Destination.Local(), os.ModePerm)
}

type WriteFileInput struct {
	Path paths.Path
	Data []byte
}

func WriteFile(ctx context.Context, input WriteFileInput) error {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "WriteFile")
	log.Info("Starting WriteFileActivity")

	err := os.MkdirAll(filepath.Dir(input.Path.Local()), os.ModePerm)
	if err != nil {
		return err
	}
	err = os.WriteFile(input.Path.Local(), input.Data, os.ModePerm)
	if err != nil {
		return err
	}
	_ = os.Chmod(input.Path.Local(), os.ModePerm)
	return nil
}

func ReadFile(ctx context.Context, input FileInput) ([]byte, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "ReadFile")
	log.Info("Starting ReadFileActivity")

	return os.ReadFile(input.Path.Local())
}

func ListFiles(ctx context.Context, input FileInput) ([]paths.Path, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "ListFiles")
	log.Info("Starting ListFilesActivity")

	files, err := filepath.Glob(filepath.Join(input.Path.Local(), "*"))
	if err != nil {
		return nil, err
	}
	return lo.Map(files, func(i string, _ int) paths.Path {
		return paths.MustParse(i)
	}), err
}

func DeletePath(ctx context.Context, input FileInput) error {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "DeletePath")
	log.Info("Starting DeletePathActivity")

	return os.RemoveAll(input.Path.Local())
}
