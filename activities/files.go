package activities

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/samber/lo"
	"go.temporal.io/sdk/activity"
)

type FileInput struct {
	Path paths.Path
}
type DeletePathInput struct {
	Path      paths.Path
	RemoveAll bool
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
		_, err = copyFile(ctx, input.Source, input.Destination)
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
	_, err = copyFile(ctx, input.Source, input.Destination)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(input.Destination.Local(), os.ModePerm)
	return &FileResult{
		Path: input.Destination,
	}, nil
}

func copyFile(ctx context.Context, source paths.Path, destination paths.Path) (any, error) {
	log := activity.GetLogger(ctx)
	sourcePath := source.Local()
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	outputFile, err := os.Create(destination.Local())
	if err != nil {
		closeErr := inputFile.Close()
		if closeErr != nil {
			log.Error(err.Error())
		}
		return nil, err
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
	return nil, err
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

func CreateFolder(ctx context.Context, input CreateFolderInput) (any, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "CreateFolder")
	log.Info("Starting CreateFolderActivity")

	err := os.MkdirAll(input.Destination.Local(), os.ModePerm)
	if err != nil {
		return nil, err
	}
	return nil, os.Chmod(input.Destination.Local(), os.ModePerm)
}

type WriteFileInput struct {
	Path paths.Path
	Data []byte
}

func WriteFile(ctx context.Context, input WriteFileInput) (any, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "WriteFile")
	log.Info("Starting WriteFileActivity")

	stop := simpleHeartBeater(ctx)
	defer close(stop)

	err := os.MkdirAll(filepath.Dir(input.Path.Local()), os.ModePerm)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(input.Path.Local(), input.Data, os.ModePerm)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(input.Path.Local(), os.ModePerm)
	return nil, nil
}

func ReadFile(ctx context.Context, input FileInput) ([]byte, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "ReadFile")
	log.Info("Starting ReadFileActivity")

	return os.ReadFile(input.Path.Local())
}

func ListFiles(ctx context.Context, input FileInput) (paths.Files, error) {
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

func DeletePath(ctx context.Context, input DeletePathInput) (any, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "DeletePath")
	log.Info("Starting DeletePathActivity")

	if (input.Path.Path == "/") || (input.Path.Path == "") {
		return nil, merry.New("cannot delete root")
	}

	if input.RemoveAll {
		return nil, os.RemoveAll(input.Path.Local())
	}

	return nil, os.Remove(input.Path.Local())
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
			if time.Since(startedAt) > time.Minute*30 {
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
		time.Sleep(time.Second * 5)
	}
}

type CleanupInput struct {
	Root      paths.Path
	OlderThan time.Time
}

func DeleteEmptyDirectories(ctx context.Context, input CleanupInput) ([]string, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "DeleteEmptyDirectories")
	log.Info("Starting DeleteEmptyDirectoriesActivity")

	empty, err := utils.GetEmptyDirs(input.Root.Local())
	if err != nil {
		return nil, err
	}

	deleted := []string{}
	for _, dir := range empty {
		err := os.Remove(dir)
		if err != nil {
			return deleted, err
		}
		deleted = append(deleted, dir)
	}

	return deleted, nil
}

func DeleteOldFiles(ctx context.Context, input CleanupInput) ([]string, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "DeleteOldFiles")
	log.Info("Starting DeleteOldFilesActivity")

	deleted := []string{}
	files, err := utils.GetOldFiles(input.Root.Local(), input.OlderThan)
	if err != nil {
		return deleted, err
	}

	for _, file := range files {
		err := os.Remove(file)
		if err != nil {
			return deleted, err
		}
		deleted = append(deleted, file)
	}

	return deleted, err
}
