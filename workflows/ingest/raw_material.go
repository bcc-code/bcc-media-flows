package ingest

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/activities"
	vsactivity "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"github.com/bcc-code/bccm-flows/workflows"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
	"path/filepath"
	"strings"
)

type RawMaterialParams struct {
	Job   common.IngestJob
	Files []utils.Path
}

func RawMaterial(ctx workflow.Context, params RawMaterialParams) error {
	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	outputFolder, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return err
	}
	outputPath, err := utils.ParsePath(outputFolder)
	if err != nil {
		return err
	}

	var fileByFilename = map[string]utils.Path{}
	for _, f := range params.Files {
		fileName := filepath.Base(f.FileName())
		fileByFilename[fileName] = f
		if !utils.ValidRawFilename(fileName) {
			return fmt.Errorf("invalid filename: %s, %s", f.Drive, f.Path)
		}
	}

	for _, f := range params.Files {
		err = workflow.ExecuteActivity(ctx, activities.RcloneMoveFileActivity, activities.RcloneMoveFileInput{
			Source:      f,
			Destination: outputPath.Append(filepath.Base(f.Path)),
		}).Get(ctx, nil)
		if err != nil {
			return err
		}
	}

	files, err := wfutils.ListFiles(ctx, outputFolder)
	if err != nil {
		return err
	}

	var assetAnalyzeTasks = map[string]workflow.Future{}

	var vidispineJobIDs = map[string]string{}

	for _, file := range files {
		f, found := lo.Find(params.Files, func(f utils.Path) bool {
			return f.FileName() == filepath.Base(file)
		})
		if !found {
			return fmt.Errorf("file not found: %s", file)
		}
		var result vsactivity.CreatePlaceholderResult
		err = workflow.ExecuteActivity(ctx, vsactivity.CreatePlaceholderActivity, vsactivity.CreatePlaceholderParams{
			Title: f.FileName(),
		}).Get(ctx, &result)
		if err != nil {
			return err
		}
		var job vsactivity.JobResult
		err = workflow.ExecuteActivity(ctx, vsactivity.ImportFileAsShapeActivity, vsactivity.ImportFileAsShapeParams{
			AssetID:  result.AssetID,
			FilePath: file,
			ShapeTag: "original",
		}).Get(ctx, &job)

		if err != nil {
			return err
		}
		vidispineJobIDs[result.AssetID] = job.JobID

		assetAnalyzeTasks[result.AssetID] = workflow.ExecuteActivity(ctx, activities.AnalyzeFile, activities.AnalyzeFileParams{
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

		err = wfutils.SetVidispineMeta(ctx, id, vscommon.FieldUploadedBy.Value, strings.Join(params.Job.SenderEmails, ", "))
		if err != nil {
			return err
		}

		err = wfutils.SetVidispineMeta(ctx, id, vscommon.FieldUploadJob.Value, params.Job.JobID)
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
			VXID: id,
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
