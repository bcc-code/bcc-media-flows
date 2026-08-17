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

func CleanupTemp(ctx workflow.Context) (*CleanupResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting temp files cleanup")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	olderThan := workflow.Now(ctx).Add(-14 * 24 * time.Hour)

	folders := []string{
		"/mnt/temp/",
		"/mnt/filecatalyst/ingestgrow/",
		"/mnt/filecatalyst/workflow/",
		"/mnt/isilon/Input/FromArvoll",
		"/mnt/isilon/Input/FromDelivery",
		"/mnt/isilon/Input/MGOF",
		"/mnt/isilon/Input/Rawmaterial",

		// Transcoding folders
		"/mnt/isilon/Transcoding/AVCintra100_HD/error",
		"/mnt/isilon/Transcoding/AVCintra100_HD/out",
		"/mnt/isilon/Transcoding/AVCintra100_HD/processed",
		"/mnt/isilon/Transcoding/AVCintra100_HD/processing",
		"/mnt/isilon/Transcoding/AVCintra100_HD/tmp",

		"/mnt/isilon/Transcoding/AVCIntra100_TCSet/In",
		"/mnt/isilon/Transcoding/AVCIntra100_TCSet/Out",

		"/mnt/isilon/Transcoding/BroadcastWav_withTC/In",
		"/mnt/isilon/Transcoding/BroadcastWav_withTC/Out",

		"/mnt/isilon/Transcoding/Fallback/In",
		"/mnt/isilon/Transcoding/Fallback/Out",

		"/mnt/isilon/Transcoding/ImageSequence/Input",
		"/mnt/isilon/Transcoding/ImageSequence/Out",

		"/mnt/isilon/Transcoding/IMX50/In",
		"/mnt/isilon/Transcoding/IMX50/Out",

		"/mnt/isilon/Transcoding/Multitrack_Playback/Input",
		"/mnt/isilon/Transcoding/Multitrack_Playback/Output",

		"/mnt/isilon/Transcoding/ProRes422D/in",

		"/mnt/isilon/Transcoding/ProRes422HQ_HD/error",
		"/mnt/isilon/Transcoding/ProRes422HQ_HD/out",
		"/mnt/isilon/Transcoding/ProRes422HQ_HD/processed",
		"/mnt/isilon/Transcoding/ProRes422HQ_HD/processing",
		"/mnt/isilon/Transcoding/ProRes422HQ_HD/tmp",

		"/mnt/isilon/Transcoding/ProRes422HQ_HD_16chaudio/In",
		"/mnt/isilon/Transcoding/ProRes422HQ_HD_16chaudio/Out",

		"/mnt/isilon/Transcoding/ProRes422HQ_Native/error",
		"/mnt/isilon/Transcoding/ProRes422HQ_Native/out",
		"/mnt/isilon/Transcoding/ProRes422HQ_Native/processed",
		"/mnt/isilon/Transcoding/ProRes422HQ_Native/processing",
		"/mnt/isilon/Transcoding/ProRes422HQ_Native/tmp",

		"/mnt/isilon/Transcoding/ProRes422HQ_Native_25FPS/error",
		"/mnt/isilon/Transcoding/ProRes422HQ_Native_25FPS/out",
		"/mnt/isilon/Transcoding/ProRes422HQ_Native_25FPS/processed",
		"/mnt/isilon/Transcoding/ProRes422HQ_Native_25FPS/processing",
		"/mnt/isilon/Transcoding/ProRes422HQ_Native_25FPS/tmp",

		"/mnt/isilon/Transcoding/ProRes444_4K-25FPS/In",
		"/mnt/isilon/Transcoding/ProRes444_4K-25FPS/Out",

		"/mnt/isilon/Transcoding/SRT_TCOffset/In",
		"/mnt/isilon/Transcoding/SRT_TCOffset/Out",

		"/mnt/isilon/Transcoding/tmp",

		"/mnt/isilon/Transcoding/Transcribe/error",
		"/mnt/isilon/Transcoding/Transcribe/out",
		"/mnt/isilon/Transcoding/Transcribe/processed",
		"/mnt/isilon/Transcoding/Transcribe/processing",
		"/mnt/isilon/Transcoding/Transcribe/tmp",

		"/mnt/isilon/Transcoding/Wav/In",
		"/mnt/isilon/Transcoding/Wav/Out",

		"/mnt/isilon/Transcoding/XDCAMHD422/In",
		"/mnt/isilon/Transcoding/XDCAMHD422/Out",

		"/mnt/isilon/Export",
	}

	deletedTotal := 0
	deletedPerRoot := map[string]int{}

	for _, folder := range folders {
		deletedFilesLoop, err := wfutils.ExecuteWithLowPrioQueue(ctx, activities.Util.DeleteOldFiles, activities.CleanupInput{
			Root:      paths.MustParse(folder),
			OlderThan: olderThan,
		}).Result(ctx)

		if err != nil {
			logger.Error("Error during temp files cleanup", "error", err)
			return nil, err
		}

		logger.Info("Deleted files", "root", folder, "count", len(deletedFilesLoop))

		deletedPerRoot[folder] = len(deletedFilesLoop)
		deletedTotal += len(deletedFilesLoop)

		err = wfutils.ExecuteWithLowPrioQueue(ctx, activities.Util.DeleteEmptyDirectories, activities.CleanupInput{
			Root: paths.MustParse(folder),
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
