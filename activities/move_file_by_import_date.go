package activities

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	avidispine "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	cantemoservice "github.com/bcc-code/bcc-media-flows/services/cantemo"
)

var regIllegalChars = regexp.MustCompile(`([^a-zA-Z0-9\-._]|\s)`)

type MoveFileByImportDateParams struct {
	SourceStorageID      string
	DestinationStorageID string
	FileName             string
}

func (ua UtilActivities) MoveFileByImportDate(ctx context.Context, params MoveFileByImportDateParams) (any, error) {
	storageID := params.SourceStorageID
	fileName := params.FileName

	if params.SourceStorageID != params.DestinationStorageID {
		return nil, errors.New("not implemented: moving files between different storages")
	}

	filesResult, err := Cantemo.GetFiles(ctx, cantemo.GetFilesParams{
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
		if err != nil {
			return nil, err
		}

		_, err = Cantemo.RenameFile(ctx, renameData)
		return nil, err
	}

	return nil, nil
}

func generateRenameParams(ctx context.Context, file cantemoservice.Objects, oldName, prefix, oldStorage string) (*cantemo.RenameFileParams, error) {
	formats, err := Cantemo.GetFormats(ctx, cantemo.GetFormatsParams{
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
	shapes, err := Vidispine.GetShapes(ctx, avidispine.VXOnlyParam{
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
