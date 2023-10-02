package workflows

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/activities"
	vsactivity "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ingest"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
	"path/filepath"
	"strconv"
	"strings"
)

type AssetIngestParams struct {
	XMLPath string
}

type AssetIngestResult struct{}

type assetFile struct {
	Directory string
	FileName  string
}

const fcWorkflowRcloneRoot = "dmz:dmzshare/workflow"

func AssetIngest(ctx workflow.Context, params AssetIngestParams) (*AssetIngestResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting AssetIngest")

	options := GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	metadata, err := wfutils.UnmarshalXMLFile[ingest.Metadata](ctx, params.XMLPath)
	if err != nil {
		return nil, err
	}

	job := common.IngestJob{
		JobID:        strconv.Itoa(metadata.JobProperty.JobID),
		SenderEmails: strings.Split(metadata.JobProperty.SenderEmail, ","),
	}

	var directories []string
	for _, file := range metadata.FileList.Files {
		p := file.FilePath
		if !lo.Contains(directories, p) {
			directories = append(directories, p)
		}
	}

	switch metadata.JobProperty.OrderForm {
	case "Rawmaterial":
		files := lo.Map(metadata.FileList.Files, func(file ingest.File, _ int) assetFile {
			return assetFile{
				Directory: file.FilePath,
				FileName:  file.FileName,
			}
		})
		err = assetIngestRawMaterial(ctx, AssetIngestRawMaterialParams{
			Job:         job,
			Files:       files,
			Directories: directories,
		})
	}

	return &AssetIngestResult{}, nil
}

type AssetIngestRawMaterialParams struct {
	Job         common.IngestJob
	Directories []string
	Files       []assetFile
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
		path := f.Directory
		fileByFilename[f.FileName] = f
		if !utils.ValidRawFilename(f.FileName) {
			return fmt.Errorf("invalid filename: %s", path)
		}
	}

	for _, d := range params.Directories {
		err = workflow.ExecuteActivity(ctx, activities.RcloneCopyDir, activities.RcloneCopyDirInput{
			Source: filepath.Join(fcWorkflowRcloneRoot, d),
			Destination: filepath.Join(
				strings.Replace(outputFolder, utils.GetIsilonPrefix(), "isilon:isilon", 1),
			),
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

	var vidispineJobIDs []string

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
		var job vsactivity.JobResult
		err = workflow.ExecuteActivity(ctx, vsactivity.ImportFileAsShapeActivity, vsactivity.ImportFileAsShapeParams{
			AssetID:  result.AssetID,
			FilePath: file,
			ShapeTag: "original",
		}).Get(ctx, &job)

		if err != nil {
			return err
		}
		vidispineJobIDs = append(vidispineJobIDs, job.JobID)

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

		err = workflow.ExecuteActivity(ctx, vsactivity.SetVXMetadataFieldActivity, vsactivity.SetVXMetadataFieldParams{
			VXID:  id,
			Key:   vscommon.FieldUploadedBy.Value,
			Value: strings.Join(params.Job.SenderEmails, ", "),
		}).Get(ctx, nil)
		if err != nil {
			return err
		}

		err = workflow.ExecuteActivity(ctx, vsactivity.SetVXMetadataFieldActivity, vsactivity.SetVXMetadataFieldParams{
			VXID:  id,
			Key:   vscommon.FieldUploadJob.Value,
			Value: params.Job.JobID,
		}).Get(ctx, nil)
		if err != nil {
			return err
		}
	}

	for _, id := range vidispineJobIDs {
		err = wfutils.WaitForVidispineJob(ctx, id)
		if err != nil {
			return err
		}
	}

	for _, id := range assetIDs {
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
