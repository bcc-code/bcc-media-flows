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
		"/mnt/temp/":                     workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/filecatalyst/ingestgrow/":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/filecatalyst/workflow/":    workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Input/FromArvoll":   workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Input/FromDelivery": workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Input/MGOF":         workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Input/Rawmaterial":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		// Transcoding folders
		"/mnt/isilon/Transcoding/AVCintra100_HD/error":       workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/AVCintra100_HD/out":         workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/AVCintra100_HD/processed":   workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/AVCintra100_HD/processing":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/AVCintra100_HD/tmp":         workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/AVCIntra100_TCSet/In":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/AVCIntra100_TCSet/Out": workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/BroadcastWav_withTC/In":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/BroadcastWav_withTC/Out": workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/Fallback/In":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/Fallback/Out": workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/ImageSequence/Input": workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/ImageSequence/Out":   workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/IMX50/In":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/IMX50/Out": workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/Multitrack_Playback/Input":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/Multitrack_Playback/Output": workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/ProRes422D/in": workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/ProRes422HQ_HD/error":      workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/ProRes422HQ_HD/out":        workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/ProRes422HQ_HD/processed":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/ProRes422HQ_HD/processing": workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/ProRes422HQ_HD/tmp":        workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/ProRes422HQ_HD_16chaudio/In":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/ProRes422HQ_HD_16chaudio/Out": workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/ProRes422HQ_Native/error":      workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/ProRes422HQ_Native/out":        workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/ProRes422HQ_Native/processed":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/ProRes422HQ_Native/processing": workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/ProRes422HQ_Native/tmp":        workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/ProRes422HQ_Native_25FPS/error":      workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/ProRes422HQ_Native_25FPS/out":        workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/ProRes422HQ_Native_25FPS/processed":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/ProRes422HQ_Native_25FPS/processing": workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/ProRes422HQ_Native_25FPS/tmp":        workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/ProRes444_4K-25FPS/In":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/ProRes444_4K-25FPS/Out": workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/SRT_TCOffset/In":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/SRT_TCOffset/Out": workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/tmp": workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/Transcribe/error":      workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/Transcribe/out":        workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/Transcribe/processed":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/Transcribe/processing": workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/Transcribe/tmp":        workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/Wav/In":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/Wav/Out": workflow.Now(ctx).Add(-14 * 24 * time.Hour),

		"/mnt/isilon/Transcoding/XDCAMHD422/In":  workflow.Now(ctx).Add(-14 * 24 * time.Hour),
		"/mnt/isilon/Transcoding/XDCAMHD422/Out": workflow.Now(ctx).Add(-14 * 24 * time.Hour),
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
