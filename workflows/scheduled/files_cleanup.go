package scheduled

import (
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type CleanupResult struct {
	DeletedFiles []string
	DeletedCount int
}

func CleanupTemp(ctx workflow.Context) (*CleanupResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting temp files cleanup")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	foldersToCleanup := map[string]time.Time{
		"/mnt/temp/":                    workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/filecatalyst/ingestgrow/": workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/filecatalyst/workflow/":   workflow.Now(ctx).Add(-14 * 24 * time.Hour),
	}

	deletedFiles := []string{}

	folders, err := wfutils.GetMapKeysSafely(ctx, foldersToCleanup)
	if err != nil {
		return nil, err
	}

	for _, folder := range folders {
		olderThan := foldersToCleanup[folder]

		deletedFilesLoop := []string{}
		err := wfutils.ExecuteWithLowPrioQueue(ctx, activities.Util.DeleteOldFiles, activities.CleanupInput{
			Root:      paths.MustParse(folder),
			OlderThan: olderThan,
		}).Get(ctx, &deletedFilesLoop)

		logger.Info("Deleted files", "count", len(deletedFiles))

		if err != nil {
			logger.Error("Error during temp files cleanup", "error", err)
			return nil, err
		}

		deletedFiles = append(deletedFiles, deletedFilesLoop...)

		err = wfutils.ExecuteWithLowPrioQueue(ctx, activities.Util.DeleteEmptyDirectories, activities.CleanupInput{
			Root: paths.MustParse(folder),
		}).Get(ctx, nil)

		if err != nil {
			return nil, err
		}

	}

	res := &CleanupResult{
		DeletedFiles: deletedFiles,
		DeletedCount: len(deletedFiles),
	}

	return res, nil
}
