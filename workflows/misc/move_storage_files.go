package miscworkflows

import (
	"fmt"
	"sort"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
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
	Files       int
	Bytes       int64
	Failed      int
	FailedPaths []string
	SamplePaths []string
}

// MoveStorageFiles moves every item file on a storage, or under a folder of
// it, to another storage, one file at a time. The relative path is kept.
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
	prevAttempted := map[string]bool{}
	for _, id := range params.Attempted {
		prevAttempted[id] = true
	}

	for {
		if workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
			logger.Info("History is long, continuing as new", "offset", offset, "files", progress.Files)
			next := params
			next.Progress = progress
			next.Offset = offset
			next.Attempted = mapKeys(prevAttempted)
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

		if params.DryRun {
			countFiles(&progress, page.Items)
			offset += page.Files
			continue
		}

		attempted := map[string]bool{}
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
			attempted[f.FileID] = true

			err := wfutils.Execute(moveCtx, activities.Cantemo.MoveFileWait, &cantemo.RenameFileParams{
				ItemID:            f.ItemID,
				ShapeID:           f.ShapeID,
				SourceStorage:     src.VXID,
				DestinatinStorage: dst.VXID,
				NewPath:           f.Path,
			}).Wait(ctx)
			if err != nil {
				logger.Error("Failed to move file", "error", err, "path", f.Path, "itemID", f.ItemID)
				recordFailure(&progress, f.Path)
				skipped[f.FileID] = true
				continue
			}
			countFiles(&progress, []vsactivity.StorageItemFile{f})
		}

		offset += len(skipped)
		prevAttempted = attempted
	}

	logger.Info("Storage move done", "dryRun", params.DryRun, "files", progress.Files, "failed", progress.Failed)
	return &progress, nil
}

func countFiles(progress *MoveStorageFilesResult, files []vsactivity.StorageItemFile) {
	for _, f := range files {
		progress.Files++
		progress.Bytes += f.Size
		if len(progress.SamplePaths) < moveStorageFilesMaxPaths {
			progress.SamplePaths = append(progress.SamplePaths, f.Path)
		}
	}
}

func recordFailure(progress *MoveStorageFilesResult, path string) {
	progress.Failed++
	if len(progress.FailedPaths) < moveStorageFilesMaxPaths {
		progress.FailedPaths = append(progress.FailedPaths, path)
	}
}

// mapKeys is sorted: the keys go into the continue-as-new input, and map
// order would make that input differ between runs of the same history.
func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
