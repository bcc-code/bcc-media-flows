package miscworkflows

import (
	"context"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	avidispine "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	cantemoservice "github.com/bcc-code/bcc-media-flows/services/cantemo"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
	"path"
	"regexp"
	"strings"
	"time"
)

var regIllegalChars = regexp.MustCompile(`([^a-zA-Z0-9\-._]|\s)`)

type SortFilesByImportedDateParams struct {
	SourceStorageID      string
	DestinationStorageID string
	FileList             []string
	BatchSize            int
}

// SortFilesByImportedDate takes a list of files and moves them to a new location based on the date they were imported.
// The files are moved in batches of BatchSize.
//
// This workflow is intentionally not registered anywhere, as it is not meant to be used in normal day-to-day operations
// without modification and testing.
//
//workflowcheck:ignore
func SortFilesByImportedDate(
	ctx workflow.Context,
	params SortFilesByImportedDateParams,
) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting SortFilesByImportedDate")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	// Set up some variables for calculating stats
	cnt := 1
	total := len(params.FileList)
	failed := map[string]error{}
	start := workflow.Now(ctx)

	if params.BatchSize < 1 {
		params.BatchSize = 1
	}

	filesChan := lo.SliceToChannel(params.BatchSize, params.FileList)

	for {
		items, _, _, ok := lo.Buffer[string](filesChan, params.BatchSize)

		if !ok {
			break
		}

		jobs := map[string]wfutils.Task[any]{}
		for _, fileName := range items {
			fileName := strings.TrimPrefix(fileName, "./")
			j := wfutils.Execute(ctx, MoveFileByImportDate, MoveFileByImportDateParams{
				SourceStorageID:      params.SourceStorageID,
				DestinationStorageID: params.DestinationStorageID,
				FileName:             fileName,
			})

			jobs[fileName] = j
			cnt++
		}

		for k, j := range jobs {
			err := j.Wait(ctx)
			if err != nil {
				failed[k] = err
			}
		}

		fmt.Printf("Processed %d/%d %0.2f%%", cnt, total, float64(cnt)/float64(total)*100)
		fmt.Printf("Elapsed time %s", time.Since(start))
		fmt.Printf("Estimated time %0.2f", time.Since(start).Seconds()/float64(cnt)*float64(total-cnt))
		fmt.Printf("Average time %0.2f seconds per file", time.Since(start).Seconds()/float64(cnt))
	}

	for k, v := range failed {
		fmt.Printf("%s; %s", k, v.Error())
	}

	return nil
}

type MoveFileByImportDateParams struct {
	SourceStorageID      string
	DestinationStorageID string
	FileName             string
}

func MoveFileByImportDate(ctx context.Context, params MoveFileByImportDateParams) (any, error) {
	storageID := params.SourceStorageID
	fileName := params.FileName

	if params.SourceStorageID != params.DestinationStorageID {
		return nil, fmt.Errorf("not implemented: moving files between different storages")
	}

	filesResult, err := cantemo.GetFiles(ctx, cantemo.GetFilesParams{
		Path:     "/",
		Storages: []string{storageID},
		Page:     1,
		Query:    fileName,
	})

	if err != nil {
		return nil, err
	}

	for _, file := range filesResult.Objects {
		oldName := file.Name
		oldPath := path.Dir(file.Path)

		// We are only interested in files that are not sorted into directories
		if oldPath != "." && oldPath != "" {
			continue
		}

		renameData, err := generateRenameParams(ctx, file, oldName, "", params.SourceStorageID)

		_, err = cantemo.RenameFile(ctx, renameData)
		return nil, err
	}

	return nil, nil
}

func generateRenameParams(ctx context.Context, file cantemoservice.Objects, oldName, prefix, oldStorage string) (*cantemo.RenameFileParams, error) {
	formats, err := cantemo.GetFormats(ctx, cantemo.GetFormatsParams{
		ItemID: file.Item.ID,
	})

	if err != nil {
		return nil, err
	}

	var fileFormat *cantemoservice.Format
	for _, format := range formats {
		for _, f := range format.Files {
			if f.ID == file.ID {
				ff := format
				fileFormat = &ff
				break
			}
		}
	}

	if fileFormat == nil {
		return nil, fmt.Errorf("no format found for file %s", file.ID)
	}

	// This whole section is here in order to get the timestamp of the file import.
	// There is a timestamp on the format, but it seems to be the same everywhere
	shapes, err := activities.Vidispine.GetShapes(ctx, avidispine.VXOnlyParam{
		VXID: file.Item.ID,
	})

	if err != nil {
		return nil, err
	}

	for _, shape := range shapes.Shape {
		if shape.ID == fileFormat.ID {
			ts, err := time.Parse("2006-01-02T15:04:05.000-0700", shape.Created)
			if err != nil {
				return nil, err
			}

			file.Timestamp = ts
		}
	}

	newName := regIllegalChars.ReplaceAllString(oldName, "_")
	newPath := fmt.Sprintf("%04d/%02d/%02d/%s", file.Timestamp.Year(), file.Timestamp.Month(), file.Timestamp.Day(), prefix)

	if strings.Contains(fileFormat.Name, "low") {
		newPath = "aux/" + newPath
	}

	return &cantemo.RenameFileParams{
		NewPath:           newPath + newName,
		ItemID:            file.Item.ID,
		ShapeID:           fileFormat.ID,
		SourceStorage:     oldStorage,
		DestinatinStorage: oldStorage,
	}, nil
}
