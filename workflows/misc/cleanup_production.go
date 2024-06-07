package miscworkflows

import (
	"fmt"
	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	cantemoservice "github.com/bcc-code/bcc-media-flows/services/cantemo"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
	"path"
	"regexp"
	"strings"
)

var regIllegalChars = regexp.MustCompile(`([^a-zA-Z0-9\-\._]|\s)`)

func CleanupProduction(
	ctx workflow.Context,
) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting Cleanup Production")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	page := 90
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

		for _, file := range filesResult.Objects {
			oldName := file.Name
			oldPath := path.Dir(file.Path)

			// We are only interested in files that are not sorted into directories
			if oldPath != "." && oldPath != "" {
				continue
			}

			formats, err := wfutils.Execute(ctx, cantemo.GetFormats, cantemo.GetFormatsParams{
				ItemID: file.Item.ID,
			}).Result(ctx)

			if err != nil {
				return err
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
				return fmt.Errorf("no format found for file %s", file.ID)
			}

			newName := regIllegalChars.ReplaceAllString(oldName, "_")
			newPath := fmt.Sprintf("%04d/%02d/%02d/AUTOMOVE_", file.Timestamp.Year(), file.Timestamp.Month(), file.Timestamp.Day())

			if strings.Contains(fileFormat.Name, "low") {
				newPath = "aux/" + newPath
			}

			println(file.Item.ID, "Moving file", oldPath+oldName, "to", newPath+newName)
			_, err = wfutils.Execute(ctx, cantemo.RenameFile, cantemo.RenameFileParams{
				NewPath:   newPath + newName,
				ItemID:    file.Item.ID,
				ShapeID:   fileFormat.ID,
				StorageID: storageID,
			}).Result(ctx)

			if err != nil {
				return err
			}
		}
		page += 1

		if page > 150 {
			hasNext = false
		}
	}

	return nil
}
