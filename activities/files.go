package activities

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/bcc-code/bccm-flows/paths"
	"github.com/samber/lo"
	"go.temporal.io/sdk/activity"
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

	stop := simpleHeartBeater(ctx)
	defer close(stop)

	err := os.MkdirAll(filepath.Dir(input.Destination.Local()), os.ModePerm)
	if err != nil {
		return nil, err
	}
	if input.Source.Drive != input.Destination.Drive {
		err = copyFile(ctx, input.Source, input.Destination)
		if err != nil {
			return nil, err
		}
		err = os.Remove(input.Source.Local())
	} else {
		err = os.Rename(input.Source.Local(), input.Destination.Local())
	}
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(input.Destination.Local(), os.ModePerm)
	return &FileResult{
		Path: input.Destination,
	}, nil
}

func CopyFile(ctx context.Context, input MoveFileInput) (*FileResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "CopyFile")
	log.Info("Starting CopyFileActivity")

	stop := simpleHeartBeater(ctx)
	defer close(stop)

	err := os.MkdirAll(filepath.Dir(input.Destination.Local()), os.ModePerm)
	if err != nil {
		return nil, err
	}
	err = copyFile(ctx, input.Source, input.Destination)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(input.Destination.Local(), os.ModePerm)
	return &FileResult{
		Path: input.Destination,
	}, nil
}

func copyFile(ctx context.Context, source paths.Path, destination paths.Path) error {
	log := activity.GetLogger(ctx)
	sourcePath := source.Local()
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	outputFile, err := os.Create(destination.Local())
	if err != nil {
		closeErr := inputFile.Close()
		if closeErr != nil {
			log.Error(err.Error())
		}
		return err
	}
	defer func() {
		closeErr := outputFile.Close()
		if closeErr != nil {
			log.Error(err.Error())
		}
	}()
	_, err = io.Copy(outputFile, inputFile)
	closeErr := inputFile.Close()
	if closeErr != nil {
		log.Error(err.Error())
	}
	return err
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

	stop := simpleHeartBeater(ctx)
	defer close(stop)

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

// WaitForFile waits until a file stops growing
// Useful for waiting for a file to be fully uploaded, e.g. watch folders
// Returns true if file is fully uploaded, false if failed, e.g. file doesnt exist after 5 minutes.
func WaitForFile(ctx context.Context, input FileInput) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting WaitForFile")

	// Use to cancel if file doesnt exist still after 5 minutes
	startedAt := time.Now()

	lastKnownSize := int64(0)
	iterationsWhereSizeIsFreezed := 0

	for {
		res, err := os.Stat(input.Path.Local())
		activity.RecordHeartbeat(ctx, res)
		if err != nil {
			if time.Since(startedAt) > time.Minute*5 {
				return false, err
			}
			time.Sleep(time.Second * 5)
			continue
		}

		if res.Size() < lastKnownSize {
			return false, fmt.Errorf("file size decreased")
		} else if res.Size() > lastKnownSize {
			lastKnownSize = res.Size()
			iterationsWhereSizeIsFreezed = 0
			time.Sleep(time.Second * 5)
			continue
		}

		iterationsWhereSizeIsFreezed++

		if iterationsWhereSizeIsFreezed > 12 {
			return true, nil
		}
	}
}
