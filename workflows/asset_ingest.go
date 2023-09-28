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
	XmlPath string
}

type AssetIngestResult struct{}

func AssetIngest(ctx workflow.Context, params AssetIngestParams) (*AssetIngestResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting AssetIngest")

	options := GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	metadata, err := wfutils.UnmarshalXMLFile[ingest.Metadata](ctx, params.XmlPath)
	if err != nil {
		return nil, err
	}

	switch metadata.JobProperty.OrderForm {
	case "Rawmaterial":
		files := lo.Map(metadata.FileList.Files, func(file ingest.File, _ int) string {
			// dmz:dmzshare is the rclone path to the same files
			return strings.Replace("/fcweb", file.FilePath, "dmz:dmzshare", 1)
		})
		err = assetIngestRawMaterial(ctx, AssetIngestRawMaterialParams{
			FilePaths: files,
		})
	}

	return &AssetIngestResult{}, nil
}

type AssetIngestRawMaterialParams struct {
	FilePaths []string
}

func assetIngestRawMaterial(ctx workflow.Context, params AssetIngestRawMaterialParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting AssetIngestRawMaterial")

	options := GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	outputFolder, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return err
	}

	for _, path := range params.FilePaths {
		if !utils.ValidFilename(filepath.Base(path)) {
			return fmt.Errorf("invalid filename: %s", path)
		}
		err = workflow.ExecuteActivity(ctx, activities.RcloneCopy, activities.RcloneCopyDirInput{
			Source:      path,
			Destination: strings.Replace(outputFolder, utils.GetIsilonPrefix()+"/", "isilon:isilon/", 1),
		}).Get(ctx, nil)
		if err != nil {
			return err
		}
	}

	files, err := wfutils.ListFiles(ctx, outputFolder)
	if err != nil {
		return err
	}

	for _, file := range files {
		err = workflow.ExecuteActivity(ctx, vsactivity.ImportFileAsItemActivity, vsactivity.ImportFileAsItemParams{
			FilePath: file,
		}).Get(ctx, nil)
		if err != nil {
			return err
		}
	}

	return nil
}
