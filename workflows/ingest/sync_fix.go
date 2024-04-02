package ingestworkflows

import (
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/rclone"

	"github.com/bcc-code/bcc-media-flows/activities"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type IngestSyncFixParams struct {
	VXID       string
	Adjustment int
}

func IngestSyncFix(ctx workflow.Context, params IngestSyncFixParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting IngestSyncFix workflow")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	_ = wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("🟦 `%s`\n\nApplying adjustments to audio files.\n%dms", params.VXID, params.Adjustment))

	audioPaths, err := wfutils.Execute(ctx, activities.Vidispine.GetRelatedAudioFiles, params.VXID).Result(ctx)
	if err != nil {
		return err
	}

	languages, err := wfutils.GetMapKeysSafely(ctx, audioPaths)
	if err != nil {
		return err
	}

	outputFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return err
	}

	var errs []error
	selector := workflow.NewSelector(ctx)
	for _, lang := range languages {
		path := audioPaths[lang]
		dest := outputFolder.Append(path.Base())
		jobID, err := wfutils.Execute(ctx, activities.Util.RcloneCopyFile, activities.RcloneFileInput{
			Source:      path,
			Destination: dest,
			Priority:    rclone.PriorityHigh,
		}).Result(ctx)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		f := wfutils.Execute(ctx, activities.Util.RcloneWaitForJob, jobID).Future
		selector.AddFuture(f, func(future workflow.Future) {
			var copied bool
			err := future.Get(ctx, &copied)
			if err != nil {
				errs = append(errs, err)
				return
			}
			if !copied {
				errs = append(errs, fmt.Errorf("failed to copy file %s to %s", path, dest))
				return
			}
			var f workflow.Future
			samples := params.Adjustment / 1000 * 48000
			if samples > 0 {
				f = wfutils.Execute(ctx, activities.Audio.PrependSilence, activities.PrependSilenceInput{
					FilePath:   dest,
					Output:     path,
					SampleRate: 48000,
					Samples:    samples,
				}).Future
			} else {
				f = wfutils.Execute(ctx, activities.Audio.TrimFile, activities.TrimInput{
					Input:  dest,
					Output: path,
					Start:  float64(-samples) / float64(48000),
				}).Future
			}
			selector.AddFuture(f, func(future workflow.Future) {
				err := future.Get(ctx, nil)
				if err != nil {
					errs = append(errs, err)
				}
			})
		})
	}

	for range languages {
		selector.Select(ctx)
		selector.Select(ctx)
	}

	_ = wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("🟩 `%s`\n\nAdjustments applied to audio files.", params.VXID))

	return nil
}
