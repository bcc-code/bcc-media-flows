package miscworkflows

import (
	"errors"
	"fmt"
	"sort"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/fcplibrary"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type CopyVXWithRelatedParams struct {
	VXID string
}

type CopyVXWithRelatedResult struct {
	Folder string
	// Library is the restored FCP library, when the item is a Cantemo Portal FCP library.
	Library string `json:",omitempty"`
	Copied  []string
	// Skipped lists the VX-IDs that have no original file.
	Skipped []string
	// LibrarySkipped lists portalproject.xml entries that could not be placed in the library.
	LibrarySkipped []fcplibrary.SkippedItem `json:",omitempty"`
}

// copyVXParallelism caps the lookups and copies running at the same time.
const copyVXParallelism = 8

var errCopyVXWithRelatedMissingVXID = errors.New("VXID is required")

// CopyVXWithRelated copies the original file of an item, and of every item directly
// related to it, into a new Isilon export folder.
//
// When the item is an FCP library stored by Cantemo Portal (.fcpbundlezip), the library
// is extracted into the folder instead of copied, and each media item its
// portalproject.xml lists is copied to its path inside the library, so the library
// opens with its media online. Related items the library does not list are copied
// next to it.
func CopyVXWithRelated(ctx workflow.Context, params CopyVXWithRelatedParams) (*CopyVXWithRelatedResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting CopyVXWithRelated", "vxid", params.VXID)

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	if params.VXID == "" {
		return nil, errCopyVXWithRelatedMissingVXID
	}

	result, err := copyVXWithRelated(ctx, params.VXID)
	if err != nil {
		wfutils.SendTelegramText(ctx, telegram.ChatOther,
			fmt.Sprintf("🟥 Copy of `%s` with related items failed: ```%v```", params.VXID, err))
		return nil, err
	}

	msg := fmt.Sprintf("🟩 Copied `%s` with related items.\nFolder: `%s`\nCopied: %d, skipped: %d",
		params.VXID, result.Folder, len(result.Copied), len(result.Skipped))
	if result.Library != "" {
		msg += fmt.Sprintf("\nFCP library: `%s` (%d project entries not placed)", result.Library, len(result.LibrarySkipped))
	}
	wfutils.SendTelegramText(ctx, telegram.ChatOther, msg)

	return result, nil
}

type copyJob struct {
	source      paths.Path
	destination paths.Path
}

func copyVXWithRelated(ctx workflow.Context, vxID string) (*CopyVXWithRelatedResult, error) {
	logger := workflow.GetLogger(ctx)

	relations, err := wfutils.Execute(ctx, activities.Vidispine.GetRelations, vxID).Result(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting relations of %s: %w", vxID, err)
	}
	related := relatedVXIDs(vxID, relations)

	folder := wfutils.GetIsilonExportFolder(ctx, vxID)

	wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf(
		"🟦 Copy of `%s` with %d related items started.\nFolder: `%s`\n\nRunID: `%s`",
		vxID, len(related), folder.Linux(), workflow.GetInfo(ctx).OriginalRunID,
	))

	if err := wfutils.CreateFolder(ctx, folder); err != nil {
		return nil, fmt.Errorf("creating folder %s: %w", folder.Linux(), err)
	}

	main, err := getOriginal(ctx, vxID)
	if err != nil {
		return nil, fmt.Errorf("getting original file of %s: %w", vxID, err)
	}

	result := &CopyVXWithRelatedResult{Folder: folder.Linux()}

	// Items that go into the FCP library, and the items left to copy flat into the folder.
	var libraryItems []fcplibrary.Item
	var library paths.Path
	var flat []string
	if fcplibrary.IsArchive(main.Base()) {
		extracted, err := wfutils.Execute(ctx, activities.Util.ExtractFCPLibrary, activities.ExtractFCPLibraryInput{
			Archive:     main,
			Destination: folder,
		}).Result(ctx)
		if err != nil {
			return nil, fmt.Errorf("extracting FCP library %s: %w", main.Linux(), err)
		}
		library = extracted.Library
		libraryItems = extracted.Items
		result.Library = library.Linux()
		result.LibrarySkipped = extracted.Skipped
		for _, s := range extracted.Skipped {
			logger.Warn("Project entry not placed in library", "vxid", s.VXID, "path", s.Path, "reason", s.Reason)
		}

		inLibrary := map[string]bool{}
		for _, it := range libraryItems {
			inLibrary[it.VXID] = true
		}
		flat = nil
		for _, id := range related {
			if !inLibrary[id] {
				flat = append(flat, id)
			}
		}
	} else {
		flat = append([]string{vxID}, related...)
	}

	// Look up the original file of every item, in parallel.
	var lookup []string
	seen := map[string]bool{vxID: true}
	for _, it := range libraryItems {
		if !seen[it.VXID] {
			seen[it.VXID] = true
			lookup = append(lookup, it.VXID)
		}
	}
	for _, id := range flat {
		if !seen[id] {
			seen[id] = true
			lookup = append(lookup, id)
		}
	}
	originals := map[string]paths.Path{vxID: main}
	found := make([]*paths.Path, len(lookup))
	_ = forEachLimited(ctx, len(lookup), func(ctx workflow.Context, i int) error {
		p, err := getOriginal(ctx, lookup[i])
		if err != nil {
			logger.Warn("No original shape, skipping", "vxid", lookup[i], "error", err)
			return nil
		}
		found[i] = &p
		return nil
	})
	for i, id := range lookup {
		if found[i] == nil {
			result.Skipped = append(result.Skipped, id)
			continue
		}
		originals[id] = *found[i]
	}

	// Plan the copies. Flat files keep their name unless it is taken, then get the VX-ID in front.
	var jobs []copyJob
	for _, it := range libraryItems {
		if src, ok := originals[it.VXID]; ok {
			jobs = append(jobs, copyJob{source: src, destination: library.Append(it.Path)})
		}
	}
	usedNames := map[string]bool{}
	if result.Library != "" {
		usedNames[library.Base()] = true
	}
	for _, id := range flat {
		src, ok := originals[id]
		if !ok {
			continue
		}
		name := src.Base()
		if usedNames[name] {
			name = id + "_" + name
		}
		usedNames[name] = true
		jobs = append(jobs, copyJob{source: src, destination: folder.Append(name)})
	}

	err = forEachLimited(ctx, len(jobs), func(ctx workflow.Context, i int) error {
		if err := wfutils.CopyFile(ctx, jobs[i].source, jobs[i].destination); err != nil {
			return fmt.Errorf("copying %s to %s: %w", jobs[i].source.Linux(), jobs[i].destination.Linux(), err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, j := range jobs {
		result.Copied = append(result.Copied, j.destination.Linux())
	}

	return result, nil
}

func getOriginal(ctx workflow.Context, vxID string) (paths.Path, error) {
	file, err := wfutils.Execute(ctx, activities.Vidispine.GetFileFromVXActivity, vsactivity.GetFileFromVXParams{
		VXID: vxID,
		Tags: []vsapi.ShapeTag{vsapi.ShapeTagOriginal},
	}).Result(ctx)
	if err != nil {
		return paths.Path{}, err
	}
	return file.FilePath, nil
}

// forEachLimited runs fn for 0..n-1 with at most copyVXParallelism running at once,
// waits for all of them and returns their errors joined.
func forEachLimited(ctx workflow.Context, n int, fn func(ctx workflow.Context, i int) error) error {
	sem := workflow.NewSemaphore(ctx, copyVXParallelism)
	wg := workflow.NewWaitGroup(ctx)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		if err := sem.Acquire(ctx, 1); err != nil {
			return err
		}
		wg.Add(1)
		workflow.Go(ctx, func(ctx workflow.Context) {
			defer wg.Done()
			defer sem.Release(1)
			errs[i] = fn(ctx, i)
		})
	}
	wg.Wait(ctx)
	return errors.Join(errs...)
}

// relatedVXIDs returns the other end of each relation, without duplicates or vxID itself, sorted.
func relatedVXIDs(vxID string, relations []vsapi.Relation) []string {
	seen := map[string]bool{vxID: true}
	var ids []string
	for _, r := range relations {
		other := r.Direction.Source
		if other == vxID {
			other = r.Direction.Target
		}
		if other == "" || seen[other] {
			continue
		}
		seen[other] = true
		ids = append(ids, other)
	}
	sort.Strings(ids)
	return ids
}
