package ingestworkflows

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/services/ingest"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"github.com/bcc-code/bcc-media-flows/utils"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type RawMaterialParams struct {
	OrderForm OrderForm
	Targets   []notifications.Target
	Metadata  *ingest.Metadata
	Directory paths.Path
}

func RawMaterial(ctx workflow.Context, params RawMaterialParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting RawMaterial workflow")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	outputFolder, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return err
	}

	originalFiles, err := wfutils.ListFiles(ctx, params.Directory)
	if err != nil {
		return err
	}

	var files []paths.Path
	for _, f := range originalFiles {
		if !utils.ValidRawFilename(f.Local()) {
			return fmt.Errorf("invalid filename: %s", f)
		}

		filename, err := getOrderFormFilename(params.OrderForm, f, params.Metadata.JobProperty)
		if err != nil {
			return err
		}
		newPath := outputFolder.Append(filename + f.Ext())

		err = wfutils.MoveFile(ctx, f, newPath)
		if err != nil {
			return err
		}
		files = append(files, newPath)
	}

	var fileByAssetID = map[string]paths.Path{}
	var mediaAnalyzeTasks = map[string]wfutils.Task[*ffmpeg.StreamInfo]{}
	var vidispineJobIDs = map[string]string{}

	for _, file := range files {
		var result *ImportTagResult
		result, err = ImportFileAsTag(ctx, "original", file, file.Base())
		if err != nil {
			return err
		}
		fileByAssetID[result.AssetID] = file
		vidispineJobIDs[result.AssetID] = result.ImportJobID

		err = addMetaTags(ctx, result.AssetID, params.Metadata)
		if err != nil {
			return err
		}
		if utils.IsMedia(file.Local()) {
			mediaAnalyzeTasks[result.AssetID] = wfutils.Execute(ctx, activities.Audio.AnalyzeFile, activities.AnalyzeFileParams{
				FilePath: file,
			})
		}
	}

	mediaAssetIDs, err := wfutils.GetMapKeysSafely(ctx, mediaAnalyzeTasks)
	if err != nil {
		return err
	}

	audioAssetIDs := []string{}
	videoAssetIDs := []string{}

	for _, id := range mediaAssetIDs {
		task := mediaAnalyzeTasks[id]
		var result ffmpeg.StreamInfo
		err = task.Get(ctx, &result)
		if err != nil {
			return err
		}

		// need to wait for vidispine to import the file before we can create thumbnails
		err = wfutils.WaitForVidispineJob(ctx, vidispineJobIDs[id])
		if err != nil {
			return err
		}
		// Only create thumbnails if the file has video
		if result.HasVideo {
			err = wfutils.Execute(ctx, activities.Vidispine.CreateThumbnailsActivity, vsactivity.CreateThumbnailsParams{
				AssetID: id,
			}).Get(ctx, nil)
			if err != nil {
				return err
			}
		}

		if result.HasAudio {
			audioAssetIDs = append(audioAssetIDs, id)
		}

		if result.HasVideo {
			videoAssetIDs = append(videoAssetIDs, id)
		}
	}

	err = CreatePreviews(ctx, audioAssetIDs)
	if err != nil {
		return err
	}

	err = transcribe(ctx, mediaAssetIDs, params.Metadata.JobProperty.Language)
	if err != nil {
		return err
	}

	err = notifyImportCompleted(ctx, params.Targets, params.Metadata.JobProperty.JobID, fileByAssetID)
	if err != nil {
		return err
	}

	return nil
}
