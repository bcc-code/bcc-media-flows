package ingestworkflows

import (
	"fmt"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"

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

	if params.Adjustment == 0 {
		wfutils.SendTelegramText(ctx, telegram.ChatVOD, fmt.Sprintf("🟦 `%s`\n\nCalculating automatic adjustments to audio files.", params.VXID))
		// Attempt to calculate the adjustment automatically
		shapes, err := wfutils.Execute(ctx, activities.Vidispine.GetShapes, vsactivity.VXOnlyParam{
			VXID: params.VXID,
		}).Result(ctx)

		if err != nil {
			return err
		}

		originalShape := shapes.GetShape("original")
		if originalShape == nil {
			return fmt.Errorf("original shape not found")
		}

		if len(originalShape.AudioComponent) == 0 {
			return fmt.Errorf("original shape has no audio")
		}

		originalPath, err := paths.Parse(originalShape.GetPath())
		if err != nil {
			return err
		}

		tempFolder, err := wfutils.GetWorkflowTempFolder(ctx)
		if err != nil {
			return err
		}
		prepareResult, err := wfutils.Execute(ctx, activities.Audio.PrepareForTranscription, common.AudioInput{
			Path:            originalPath,
			DestinationPath: tempFolder,
		}).Result(ctx)
		if err != nil {
			return err
		}

		reaperAudioPath := ""
		if p, ok := audioPaths["nor"]; ok {
			reaperAudioPath = p.Linux()
		} else {
			return fmt.Errorf("nor audio not found")
		}

		diff, err := wfutils.Execute(ctx, activities.Util.GetAudioDiff, activities.GetAudioDiffParams{
			ReferenceFile: prepareResult.OutputPath.Linux(),
			TargetFile:    reaperAudioPath,
		}).Result(ctx)

		if err != nil {
			return err
		}

		params.Adjustment = diff.Difference

		wfutils.SendTelegramText(ctx, telegram.ChatVOD, fmt.Sprintf("🟦 `%s`\n\nAutomatic adjustment calculated: %dms", params.VXID, params.Adjustment))
	}

	wfutils.SendTelegramText(ctx, telegram.ChatVOD, fmt.Sprintf("🟦 `%s`\n\nApplying adjustments to audio files.\n%dms", params.VXID, params.Adjustment))

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

		f := wfutils.Execute(ctx, activities.Util.RcloneWaitForJob, activities.RcloneWaitForJobInput{JobID: jobID}).Future
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
			samples := int(float64(params.Adjustment) / 1000.0 * 48000.0)
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

	wfutils.SendTelegramText(ctx, telegram.ChatVOD, fmt.Sprintf("🟩 `%s`\n\nAdjustments applied to audio files.", params.VXID))

	return nil
}
