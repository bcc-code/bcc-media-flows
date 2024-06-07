package miscworkflows

import (
	"context"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	avidispine "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	cantemoservice "github.com/bcc-code/bcc-media-flows/services/cantemo"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
	"path"
	"regexp"
	"strings"
	"time"
)

var regIllegalChars = regexp.MustCompile(`([^a-zA-Z0-9\-\._]|\s)`)

type RenameCantemoFileSpecialParams struct {
	File    cantemoservice.Objects
	OldName string
	OldPath string
	Storage string
}

func RenameCantemoFileSpecial(ctx context.Context, params RenameCantemoFileSpecialParams) (any, error) {
	file := params.File
	oldName := params.OldName
	oldPath := params.OldPath
	storageID := params.Storage

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
	newPath := fmt.Sprintf("%04d/%02d/%02d/AUTOMOVE_", file.Timestamp.Year(), file.Timestamp.Month(), file.Timestamp.Day())

	if strings.Contains(fileFormat.Name, "low") {
		newPath = "aux/" + newPath
	}

	println(file.Item.ID, "Moving file", oldPath+oldName, "to", newPath+newName)

	_, err = cantemo.RenameFile(ctx, cantemo.RenameFileParams{
		NewPath:   newPath + newName,
		ItemID:    file.Item.ID,
		ShapeID:   fileFormat.ID,
		StorageID: storageID,
	})

	return nil, err
}

func CleanupProduction(
	ctx workflow.Context,
) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting Cleanup Production")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	page := 1
	hasNext := true

	storageID := "VX-42"

	for hasNext {
		filesResult, err := wfutils.Execute(ctx, cantemo.GetFiles, cantemo.GetFilesParams{
			"/",
			"imported",
			[]string{storageID},
			page,
		}).Result(ctx)

		if err != nil {
			return err
		}

		var jobs []wfutils.Task[any]
		for _, file := range filesResult.Objects {
			oldName := file.Name
			oldPath := path.Dir(file.Path)

			// We are only interested in files that are not sorted into directories
			if oldPath != "." && oldPath != "" {
				continue
			}

			task := wfutils.Execute(ctx, RenameCantemoFileSpecial, RenameCantemoFileSpecialParams{
				File:    file,
				OldName: oldName,
				OldPath: oldPath,
				Storage: storageID,
			})

			jobs = append(jobs, task)
		}

		for _, job := range jobs {
			err := job.Wait(ctx)
			if err != nil {
				return err
			}
		}

		page++
		println(page)
	}

	return nil
}
