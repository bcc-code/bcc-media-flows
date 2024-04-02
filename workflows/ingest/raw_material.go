package ingestworkflows

import (
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/rclone"

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

type RawMaterialFormParams struct {
	OrderForm OrderForm
	Targets   []notifications.Target
	Metadata  *ingest.Metadata
	Directory paths.Path
}

func RawMaterialForm(ctx workflow.Context, params RawMaterialFormParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting RawMaterial workflow")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	if params.OrderForm != OrderFormRawMaterial {
		return fmt.Errorf("invalid order form: %s", params.OrderForm)
	}

	originalFiles, err := wfutils.ListFiles(ctx, params.Directory)
	if err != nil {
		return err
	}

	fileByAssetID, err := RawMaterial(ctx, RawMaterialParams{
		FilesToIngest:    originalFiles,
		DeliveryMetadata: params.Metadata,
		Language:         params.Metadata.JobProperty.Language,
	})
	if err != nil {
		return err
	}

	err = notifyImportCompleted(ctx, params.Targets, params.Metadata.JobProperty.JobID, fileByAssetID)
	if err != nil {
		return err
	}

	return nil
}

type RawMaterialParams struct {
	FilesToIngest    paths.Files
	DeliveryMetadata *ingest.Metadata
	Language         string
}

func RawMaterial(ctx workflow.Context, params RawMaterialParams) (map[string]paths.Path, error) {
	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	outputDir, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return nil, err
	}

	files := []paths.Path{}
	for _, f := range params.FilesToIngest {
		if !utils.ValidRawFilename(f.Local()) {
			return nil, fmt.Errorf("invalid filename: %s", f)
		}

		newPath := outputDir.Append(f.Base())
		err = wfutils.MoveFile(ctx, f, newPath, rclone.PriorityNormal)
		if err != nil {
			return nil, err
		}

		files = append(files, newPath)
	}

	var fileByAssetID = map[string]paths.Path{}
	var mediaAnalyzeTasks = map[string]wfutils.Task[*ffmpeg.StreamInfo]{}
	var vidispineJobIDs = map[string]string{}

	imported := map[string]paths.Path{}
	for _, file := range files {
		var result *ImportTagResult
		result, err = ImportFileAsTag(ctx, "original", file, file.Base())
		if err != nil {
			return imported, err
		}

		imported[result.AssetID] = file

		if params.DeliveryMetadata != nil {
			err = addMetaTags(ctx, result.AssetID, params.DeliveryMetadata)
			if err != nil {
				return imported, err
			}
		}

		fileByAssetID[result.AssetID] = file
		vidispineJobIDs[result.AssetID] = result.ImportJobID

		if utils.IsMedia(file.Local()) {
			mediaAnalyzeTasks[result.AssetID] = wfutils.Execute(ctx, activities.Audio.AnalyzeFile, activities.AnalyzeFileParams{
				FilePath: file,
			})
		}
	}

	mediaAssetIDs, err := wfutils.GetMapKeysSafely(ctx, mediaAnalyzeTasks)
	if err != nil {
		return imported, err
	}

	audioAssetIDs := []string{}
	videoAssetIDs := []string{}

	for _, id := range mediaAssetIDs {
		task := mediaAnalyzeTasks[id]
		var result ffmpeg.StreamInfo
		err = task.Get(ctx, &result)
		if err != nil {
			return imported, err
		}

		// need to wait for vidispine to import the file before we can create thumbnails
		err = wfutils.WaitForVidispineJob(ctx, vidispineJobIDs[id])
		if err != nil {
			return imported, err
		}
		// Only create thumbnails if the file has video
		if result.HasVideo {
			videoAssetIDs = append(videoAssetIDs, id)

			err = wfutils.Execute(ctx, activities.Vidispine.CreateThumbnailsActivity, vsactivity.CreateThumbnailsParams{
				AssetID: id,
			}).Get(ctx, nil)
			if err != nil {
				return imported, err
			}
		}

		if result.HasAudio {
			audioAssetIDs = append(audioAssetIDs, id)
		}
	}

	err = CreatePreviews(ctx, audioAssetIDs)
	if err != nil {
		return imported, err
	}

	err = transcribe(ctx, mediaAssetIDs, params.Language)
	return imported, err
}
