package workflows

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/services/ingest"
	"github.com/bcc-code/bccm-flows/utils"
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

	metadata, err := unmarshalXMLFile[ingest.Metadata](ctx, params.XmlPath)
	if err != nil {
		return nil, err
	}

	switch metadata.JobProperty.OrderForm {
	case "Rawmaterial":
		files := lo.Map(metadata.FileList.Files, func(file ingest.File, _ int) string {
			return strings.Replace("/fcweb", file.FilePath, "", 1)
		})
		err = workflow.ExecuteChildWorkflow(ctx, AssetIngestRawMaterial, AssetIngestRawMaterialParams{
			FilePaths: files,
		}).Get(ctx, nil)
	}

	return &AssetIngestResult{}, nil
}

type AssetIngestRawMaterialParams struct {
	FilePaths []string
}

func AssetIngestRawMaterial(ctx workflow.Context, params AssetIngestRawMaterialParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting AssetIngestRawMaterial")

	options := GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	for _, path := range params.FilePaths {
		if !utils.ValidFilename(filepath.Base(path)) {
			return fmt.Errorf("invalid filename: %s", path)
		}
		outputFolder, err := getWorkflowRawOutputFolder(ctx)
		if err != nil {
			return err
		}
		err = workflow.ExecuteActivity(ctx, activities.RcloneCopy, activities.RcloneCopyDirInput{
			Source:      "dmz:dmzshare" + path,
			Destination: strings.Replace(outputFolder, utils.GetIsilonPrefix()+"/", "isilon:isilon/", 1),
		}).Get(ctx, nil)
		if err != nil {
			return err
		}

	}
	return nil
}
