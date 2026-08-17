package scheduled

import (
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

// CleanupResult reports what a cleanup run removed.
//
// Counts rather than paths: this is the workflow's completion event, and the
// paths of a fortnight of temp files across sixty folders do not belong in the
// history. They are in the activity results and the worker logs.
type CleanupResult struct {
	DeletedCount        int
	DeletedCountPerRoot map[string]int
}

const defaultRetention = 14 * 24 * time.Hour

// cleanupFolder is one folder to sweep. A zero Retention keeps
// defaultRetention, so only a folder that needs a different one says so.
type cleanupFolder struct {
	Path      string
	Retention time.Duration
}

func (f cleanupFolder) olderThan(now time.Time) time.Time {
	if f.Retention == 0 {
		return now.Add(-defaultRetention)
	}
	return now.Add(-f.Retention)
}

func CleanupTemp(ctx workflow.Context) (*CleanupResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting temp files cleanup")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	now := workflow.Now(ctx)

	folders := []cleanupFolder{
		{Path: "/mnt/temp/"},
		{Path: "/mnt/filecatalyst/ingestgrow/"},
		{Path: "/mnt/filecatalyst/workflow/"},
		{Path: "/mnt/isilon/Input/FromArvoll"},
		{Path: "/mnt/isilon/Input/FromDelivery"},
		{Path: "/mnt/isilon/Input/MGOF"},
		{Path: "/mnt/isilon/Input/Rawmaterial"},

		// Transcoding folders
		{Path: "/mnt/isilon/Transcoding/AVCintra100_HD/error"},
		{Path: "/mnt/isilon/Transcoding/AVCintra100_HD/out"},
		{Path: "/mnt/isilon/Transcoding/AVCintra100_HD/processed"},
		{Path: "/mnt/isilon/Transcoding/AVCintra100_HD/processing"},
		{Path: "/mnt/isilon/Transcoding/AVCintra100_HD/tmp"},

		{Path: "/mnt/isilon/Transcoding/AVCIntra100_TCSet/In"},
		{Path: "/mnt/isilon/Transcoding/AVCIntra100_TCSet/Out"},

		{Path: "/mnt/isilon/Transcoding/BroadcastWav_withTC/In"},
		{Path: "/mnt/isilon/Transcoding/BroadcastWav_withTC/Out"},

		{Path: "/mnt/isilon/Transcoding/Fallback/In"},
		{Path: "/mnt/isilon/Transcoding/Fallback/Out"},

		{Path: "/mnt/isilon/Transcoding/ImageSequence/Input"},
		{Path: "/mnt/isilon/Transcoding/ImageSequence/Out"},

		{Path: "/mnt/isilon/Transcoding/IMX50/In"},
		{Path: "/mnt/isilon/Transcoding/IMX50/Out"},

		{Path: "/mnt/isilon/Transcoding/Multitrack_Playback/Input"},
		{Path: "/mnt/isilon/Transcoding/Multitrack_Playback/Output"},

		{Path: "/mnt/isilon/Transcoding/ProRes422D/in"},

		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_HD/error"},
		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_HD/out"},
		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_HD/processed"},
		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_HD/processing"},
		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_HD/tmp"},

		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_HD_16chaudio/In"},
		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_HD_16chaudio/Out"},

		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_Native/error"},
		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_Native/out"},
		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_Native/processed"},
		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_Native/processing"},
		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_Native/tmp"},

		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_Native_25FPS/error"},
		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_Native_25FPS/out"},
		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_Native_25FPS/processed"},
		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_Native_25FPS/processing"},
		{Path: "/mnt/isilon/Transcoding/ProRes422HQ_Native_25FPS/tmp"},

		{Path: "/mnt/isilon/Transcoding/ProRes444_4K-25FPS/In"},
		{Path: "/mnt/isilon/Transcoding/ProRes444_4K-25FPS/Out"},

		{Path: "/mnt/isilon/Transcoding/SRT_TCOffset/In"},
		{Path: "/mnt/isilon/Transcoding/SRT_TCOffset/Out"},

		{Path: "/mnt/isilon/Transcoding/tmp"},

		{Path: "/mnt/isilon/Transcoding/Transcribe/error"},
		{Path: "/mnt/isilon/Transcoding/Transcribe/out"},
		{Path: "/mnt/isilon/Transcoding/Transcribe/processed"},
		{Path: "/mnt/isilon/Transcoding/Transcribe/processing"},
		{Path: "/mnt/isilon/Transcoding/Transcribe/tmp"},

		{Path: "/mnt/isilon/Transcoding/Wav/In"},
		{Path: "/mnt/isilon/Transcoding/Wav/Out"},

		{Path: "/mnt/isilon/Transcoding/XDCAMHD422/In"},
		{Path: "/mnt/isilon/Transcoding/XDCAMHD422/Out"},

		{Path: "/mnt/isilon/Export"},
	}

	deletedTotal := 0
	deletedPerRoot := map[string]int{}

	for _, folder := range folders {
		deletedFilesLoop, err := wfutils.ExecuteWithLowPrioQueue(ctx, activities.Util.DeleteOldFiles, activities.CleanupInput{
			Root:      paths.MustParse(folder.Path),
			OlderThan: folder.olderThan(now),
		}).Result(ctx)

		if err != nil {
			logger.Error("Error during temp files cleanup", "error", err)
			return nil, err
		}

		logger.Info("Deleted files", "root", folder.Path, "count", len(deletedFilesLoop))

		deletedPerRoot[folder.Path] = len(deletedFilesLoop)
		deletedTotal += len(deletedFilesLoop)

		err = wfutils.ExecuteWithLowPrioQueue(ctx, activities.Util.DeleteEmptyDirectories, activities.CleanupInput{
			Root: paths.MustParse(folder.Path),
		}).Wait(ctx)

		if err != nil {
			return nil, err
		}

	}

	res := &CleanupResult{
		DeletedCount:        deletedTotal,
		DeletedCountPerRoot: deletedPerRoot,
	}

	return res, nil
}
