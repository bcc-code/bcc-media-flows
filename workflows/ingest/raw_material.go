package ingestworkflows

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/activities"
	vsactivity "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/services/ingest"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"github.com/bcc-code/bccm-flows/workflows"
	"go.temporal.io/sdk/workflow"
	"path/filepath"
	"strconv"
)

type RawMaterialParams struct {
	Metadata  *ingest.Metadata
	Directory string
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

	var files []string
	for _, f := range originalFiles {
		if !utils.ValidRawFilename(f) {
			return fmt.Errorf("invalid filename: %s", f)
		}
		newPath := filepath.Join(outputFolder, filepath.Base(f))
		err = wfutils.MoveFile(ctx, f, newPath)
		if err != nil {
			return err
		}
		files = append(files, newPath)
	}

	var assetAnalyzeTasks = map[string]workflow.Future{}
	var vidispineJobIDs = map[string]string{}

	for _, file := range files {
		var result *importTagResult
		result, err = importFileAsTag(ctx, "original", file, filepath.Base(file))
		if err != nil {
			return err
		}
		vidispineJobIDs[result.AssetID] = result.ImportJobID
		assetAnalyzeTasks[result.AssetID] = wfutils.ExecuteWithQueue(ctx, activities.AnalyzeFile, activities.AnalyzeFileParams{
			FilePath: file,
		})
	}

	assetIDs, err := wfutils.GetMapKeysSafely(ctx, assetAnalyzeTasks)
	if err != nil {
		return err
	}

	for _, id := range assetIDs {
		task := assetAnalyzeTasks[id]
		var result activities.AnalyzeFileResult
		err = task.Get(ctx, &result)
		if err != nil {
			return err
		}

		err = wfutils.SetVidispineMeta(ctx, id, vscommon.FieldUploadedBy.Value, params.Metadata.JobProperty.SenderEmail)
		if err != nil {
			return err
		}

		err = wfutils.SetVidispineMeta(ctx, id, vscommon.FieldUploadJob.Value, strconv.Itoa(params.Metadata.JobProperty.JobID))
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

	var wfFutures []workflow.ChildWorkflowFuture
	for _, id := range assetIDs {
		wfFutures = append(wfFutures, workflow.ExecuteChildWorkflow(ctx, workflows.TranscodePreviewVX, workflows.TranscodePreviewVXInput{
			VXID: id,
		}))
	}

	for _, f := range wfFutures {
		err = f.Get(ctx, nil)
		if err != nil {
			return err
		}
	}

	wfFutures = []workflow.ChildWorkflowFuture{}
	for _, id := range assetIDs {
		wfFutures = append(wfFutures, workflow.ExecuteChildWorkflow(ctx, workflows.TranscribeVX, workflows.TranscribeVXInput{
			VXID:     id,
			Language: "no", // TODO: replace with language detected from file ??
		}))
	}

	for _, f := range wfFutures {
		err = f.Get(ctx, nil)
		if err != nil {
			return err
		}
	}

	return nil
}
