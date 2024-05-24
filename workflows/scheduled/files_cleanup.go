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

func CleanupTemp(ctx workflow.Context) (CleanupResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting temp files cleanup")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	deletedFiles := []string{}
	rootPath, err := paths.SafeParse(ctx, "/mnt/temp/")
	if err != nil {
		return CleanupResult{}, err
	}

	err = wfutils.ExecuteWithLowPrioQueue(ctx, activities.Util.DeleteOldFiles, activities.CleanupInput{
		Root:      rootPath,
		OlderThan: wfutils.Now(ctx).Add(-14 * 24 * time.Hour),
	}).Get(ctx, &deletedFiles)

	logger.Info("Deleted files", "count", len(deletedFiles))

	res := CleanupResult{
		DeletedFiles: deletedFiles,
		DeletedCount: len(deletedFiles),
	}

	if err != nil {
		logger.Error("Error during temp files cleanup", "error", err)
		return res, err
	}

	temp, _ := paths.SafeParse(ctx, "/mnt/temp/")
	err = wfutils.ExecuteWithLowPrioQueue(ctx, activities.Util.DeleteEmptyDirectories, activities.CleanupInput{
		Root: temp,
	}).Get(ctx, nil)

	return res, err
}
