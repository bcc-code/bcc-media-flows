package workflows

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/activities"
	vsactivity "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/services/ingest"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
	"path/filepath"
	"strings"
)

type AssetIngestParams struct {
	XMLPath string
}

type AssetIngestResult struct{}

type assetFile struct {
	Path     string
	FileName string
}

func AssetIngest(ctx workflow.Context, params AssetIngestParams) (*AssetIngestResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting AssetIngest")

	options := GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	metadata, err := wfutils.UnmarshalXMLFile[ingest.Metadata](ctx, params.XMLPath)
	if err != nil {
		return nil, err
	}

	switch metadata.JobProperty.OrderForm {
	case "Rawmaterial":
		files := lo.Map(metadata.FileList.Files, func(file ingest.File, _ int) assetFile {
			// dmz:dmzshare is the rclone path to the same files
			return assetFile{
				Path:     strings.Replace("/fcweb", file.FilePath, "dmz:dmzshare", 1),
				FileName: file.FileName,
			}
		})
		err = assetIngestRawMaterial(ctx, AssetIngestRawMaterialParams{
			Files: files,
		})
	}

	return &AssetIngestResult{}, nil
}

type AssetIngestRawMaterialParams struct {
	Files []assetFile
}

func assetIngestRawMaterial(ctx workflow.Context, params AssetIngestRawMaterialParams) error {
	options := GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	outputFolder, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return err
	}

	var fileByFilename = map[string]assetFile{}
	for _, f := range params.Files {
		path := f.Path
		fileByFilename[f.FileName] = f
		if !utils.ValidRawFilename(filepath.Base(path)) {
			return fmt.Errorf("invalid filename: %s", path)
		}
	}

	for _, f := range params.Files {
		err = workflow.ExecuteActivity(ctx, activities.RcloneCopy, activities.RcloneCopyInput{
			Source:      f.Path,
			Destination: strings.Replace(outputFolder, utils.GetIsilonPrefix(), "isilon:isilon", 1),
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

	for _, file := range files {
		f, found := lo.Find(params.Files, func(f assetFile) bool {
			return f.FileName == filepath.Base(file)
		})
		if !found {
			return fmt.Errorf("file not found: %s", file)
		}
		var result vsactivity.CreatePlaceholderResult
		err = workflow.ExecuteActivity(ctx, vsactivity.CreatePlaceholderActivity, vsactivity.CreatePlaceholderParams{
			Title: f.FileName,
		}).Get(ctx, &result)
		if err != nil {
			return err
		}
		err = workflow.ExecuteActivity(ctx, vsactivity.ImportFileAsShapeActivity, vsactivity.ImportFileAsShapeParams{
			AssetID:  result.AssetID,
			FilePath: file,
			ShapeTag: "original",
		}).Get(ctx, nil)
		if err != nil {
			return err
		}

		assetAnalyzeTasks[result.AssetID] = workflow.ExecuteActivity(ctx, activities.AnalyzeFile, activities.AnalyzeFileParams{
			FilePath: file,
		})
	}

	assetIDs, err := wfutils.GetMapKeysSafely(ctx, assetAnalyzeTasks)
	if err != nil {
		return err
	}

	var wfFutures []workflow.ChildWorkflowFuture

	for _, id := range assetIDs {
		task := assetAnalyzeTasks[id]
		var result activities.AnalyzeFileResult
		err = task.Get(ctx, &result)
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

		wfFutures = append(wfFutures, workflow.ExecuteChildWorkflow(ctx, TranscodePreviewVX, TranscodePreviewVXInput{
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
