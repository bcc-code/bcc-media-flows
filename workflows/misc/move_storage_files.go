package miscworkflows

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/samber/lo"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	moveStorageFilesPageSize = 500
	// moveStorageFilesMaxPaths caps the sample and failure lists in the result.
	moveStorageFilesMaxPaths = 50
)

type MoveStorageFilesParams struct {
	SourceStorage      string
	DestinationStorage string
	// PathPrefix limits the move to a folder, relative to the source storage
	// root. Empty moves the whole storage.
	PathPrefix string
	// DryRun only counts what would be moved.
	DryRun bool

	// Carried across continue-as-new; zero on the first run.
	Progress MoveStorageFilesResult
	// Offset is where the next listing page starts.
	Offset int
	// Attempted holds the files of the previous page, so a file that is still
	// on the source after its move is skipped instead of retried forever.
	Attempted []string
}

type MoveStorageFilesResult struct {
	Files int
	Bytes int64
	// Redated counts the files put under YYYY/MM because they sat less than
	// minFolderDepth folders deep.
	Redated     int
	Failed      int
	FailedPaths []string
	SamplePaths []string
}

// MoveStorageFiles moves every item file on a storage, or under a folder of
// it, to another storage, one file at a time. A file at least two folders deep
// keeps its relative path; a shallower one goes under the YYYY/MM of the
// item's Cantemo creation date.
//
// Moved files leave the source listing, so the next page starts after the
// files that could not be moved rather than after the page that was read.
func MoveStorageFiles(ctx workflow.Context, params MoveStorageFilesParams) (*MoveStorageFilesResult, error) {
	logger := workflow.GetLogger(ctx)

	src := FindStorageForVXID(params.SourceStorage)
	dst := FindStorageForVXID(params.DestinationStorage)
	if src == nil || dst == nil {
		return nil, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("unknown storage: source %q, destination %q", params.SourceStorage, params.DestinationStorage),
			"UNKNOWN_STORAGE", nil)
	}
	if src.VXID == dst.VXID {
		return nil, temporal.NewNonRetryableApplicationError("source and destination storage are the same", "SAME_STORAGE", nil)
	}

	moveCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 12 * time.Hour,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: 30 * time.Second,
			MaximumAttempts: 3,
		},
	})

	progress := params.Progress
	offset := params.Offset
	prevAttemptedIDs := params.Attempted
	prevAttempted := lookup(prevAttemptedIDs)

	for {
		if workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
			logger.Info("History is long, continuing as new", "offset", offset, "files", progress.Files)
			next := params
			next.Progress = progress
			next.Offset = offset
			next.Attempted = prevAttemptedIDs
			return nil, workflow.NewContinueAsNewError(ctx, MoveStorageFiles, next)
		}

		page, err := wfutils.Execute(ctx, vsactivity.Vidispine.ListItemFilesOnStorage, vsactivity.ListItemFilesOnStorageParams{
			StorageID:  src.VXID,
			PathPrefix: params.PathPrefix,
			Offset:     offset,
			Count:      moveStorageFilesPageSize,
		}).Result(ctx)
		if err != nil {
			return nil, err
		}
		if page.Files == 0 {
			break
		}

		created := map[string]time.Time{}

		if params.DryRun {
			for _, f := range page.Items {
				newPath, err := datedDestinationPath(ctx, f, created)
				if err != nil {
					logger.Error("Failed to work out destination path", "error", err, "path", f.Path, "itemID", f.ItemID)
					recordFailure(&progress, f.Path)
					continue
				}
				countFile(&progress, f, newPath)
			}
			offset += page.Files
			continue
		}

		// A slice, not a map, so the continue-as-new input is the same on replay.
		var attemptedIDs []string
		skipped := map[string]bool{}
		for _, f := range page.Items {
			if skipped[f.FileID] {
				continue
			}
			if prevAttempted[f.FileID] {
				logger.Error("File is still on the source storage after its move", "path", f.Path, "fileID", f.FileID)
				recordFailure(&progress, f.Path)
				skipped[f.FileID] = true
				continue
			}
			if !lo.Contains(attemptedIDs, f.FileID) {
				attemptedIDs = append(attemptedIDs, f.FileID)
			}

			newPath, err := datedDestinationPath(ctx, f, created)
			if err != nil {
				logger.Error("Failed to work out destination path", "error", err, "path", f.Path, "itemID", f.ItemID)
				recordFailure(&progress, f.Path)
				skipped[f.FileID] = true
				continue
			}

			err = wfutils.Execute(moveCtx, activities.Cantemo.MoveFileWait, &cantemo.RenameFileParams{
				ItemID:            f.ItemID,
				ShapeID:           f.ShapeID,
				SourceStorage:     src.VXID,
				DestinatinStorage: dst.VXID,
				NewPath:           newPath,
			}).Wait(ctx)
			if err != nil {
				logger.Error("Failed to move file", "error", err, "path", f.Path, "itemID", f.ItemID)
				recordFailure(&progress, f.Path)
				skipped[f.FileID] = true
				continue
			}
			countFile(&progress, f, newPath)
		}

		offset += len(skipped)
		prevAttemptedIDs = attemptedIDs
		prevAttempted = lookup(attemptedIDs)
	}

	logger.Info("Storage move done", "dryRun", params.DryRun, "files", progress.Files, "failed", progress.Failed)
	return &progress, nil
}

// minFolderDepth is how many folders deep a file must sit on the destination.
// A shallower file is put under the item's creation year and month.
const minFolderDepth = 2

// datedDestinationPath keeps a path that is already minFolderDepth folders
// deep, and otherwise puts it under YYYY/MM of the item's Cantemo creation
// date. created caches the dates per item for the page.
func datedDestinationPath(ctx workflow.Context, f vsactivity.StorageItemFile, created map[string]time.Time) (string, error) {
	if strings.Count(f.Path, "/") >= minFolderDepth {
		return f.Path, nil
	}

	date, ok := created[f.ItemID]
	if !ok {
		var err error
		date, err = wfutils.Execute(ctx, activities.Cantemo.GetItemCreated, cantemo.GetItemCreatedParams{ItemID: f.ItemID}).Result(ctx)
		if err != nil {
			return "", err
		}
		created[f.ItemID] = date
	}

	return path.Join(date.Format("2006/01"), f.Path), nil
}

func countFile(progress *MoveStorageFilesResult, f vsactivity.StorageItemFile, newPath string) {
	progress.Files++
	progress.Bytes += f.Size
	if newPath != f.Path {
		progress.Redated++
	}
	if len(progress.SamplePaths) < moveStorageFilesMaxPaths {
		progress.SamplePaths = append(progress.SamplePaths, f.Path+" -> "+newPath)
	}
}

func recordFailure(progress *MoveStorageFilesResult, path string) {
	progress.Failed++
	if len(progress.FailedPaths) < moveStorageFilesMaxPaths {
		progress.FailedPaths = append(progress.FailedPaths, path)
	}
}

func lookup(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}
