package ingestworkflows

import (
	"fmt"
	"strconv"

	"github.com/bcc-code/bccm-flows/activities"
	vsactivity "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/services/ingest"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type RawMaterialParams struct {
	OrderForm OrderForm
	Metadata  *ingest.Metadata
	Directory paths.Path
}

func RawMaterial(ctx workflow.Context, params RawMaterialParams) error {
	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

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

	var mediaAnalyzeTasks = map[string]workflow.Future{}
	var vidispineJobIDs = map[string]string{}

	for _, file := range files {
		var result *importTagResult
		result, err = importFileAsTag(ctx, "original", file, file.Base())
		if err != nil {
			return err
		}
		vidispineJobIDs[result.AssetID] = result.ImportJobID

		err = addMetaTags(ctx, result.AssetID, params.Metadata)
		if err != nil {
			return err
		}
		if utils.IsMedia(file.Local()) {
			mediaAnalyzeTasks[result.AssetID] = wfutils.ExecuteWithQueue(ctx, activities.AnalyzeFile, activities.AnalyzeFileParams{
				FilePath: file,
			})
		}
	}

	mediaAssetIDs, err := wfutils.GetMapKeysSafely(ctx, mediaAnalyzeTasks)
	if err != nil {
		return err
	}

	for _, id := range mediaAssetIDs {
		task := mediaAnalyzeTasks[id]
		var result activities.AnalyzeFileResult
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
			err = workflow.ExecuteActivity(ctx, vsactivity.CreateThumbnailsActivity, vsactivity.CreateThumbnailsParams{
				AssetID: id,
			}).Get(ctx, nil)
			if err != nil {
				return err
			}
		}
	}

	err = transcodeAndTranscribe(ctx, mediaAssetIDs, params.Metadata.JobProperty.Language)
	if err != nil {
		return err
	}

	return nil
}
