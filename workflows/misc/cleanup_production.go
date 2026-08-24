package miscworkflows

import (
	"fmt"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
)

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
			j := wfutils.Execute(ctx, activities.Util.MoveFileByImportDate, activities.MoveFileByImportDateParams{
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
