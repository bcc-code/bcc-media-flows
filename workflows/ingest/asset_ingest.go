package ingestworkflows

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/services/ingest"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"github.com/orsinium-labs/enum"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
	"path/filepath"
)

type OrderForm enum.Member[string]

var (
	OrderFormRawMaterial  = OrderForm{Value: "Rawmaterial"}
	OrderFormVBMaster     = OrderForm{Value: "VB"}
	OrderFormSeriesMaster = OrderForm{Value: "Series"}
	OrderForms            = enum.New(
		OrderFormRawMaterial,
		//OrderFormVBMaster, // commented out for supporting only raw material
	)
)

type AssetParams struct {
	XMLPath string
}

type AssetResult struct{}

func Asset(ctx workflow.Context, params AssetParams) (*AssetResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting Asset")

	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	metadata, err := wfutils.UnmarshalXMLFile[ingest.Metadata](ctx, params.XMLPath)
	if err != nil {
		return nil, err
	}

	orderForm := OrderForms.Parse(metadata.JobProperty.OrderForm)
	if orderForm == nil {
		return nil, fmt.Errorf("unsupported order form: %s", metadata.JobProperty.OrderForm)
	}
	_, err = wfutils.MoveToFolder(ctx,
		params.XMLPath,
		filepath.Join(filepath.Dir(params.XMLPath), "processed"),
	)
	if err != nil {
		return nil, err
	}

	tempDir, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	fcOutputDir := filepath.Join(tempDir, "fc")
	err = wfutils.CreateFolder(ctx, fcOutputDir)
	if err != nil {
		return nil, err
	}

	err = copyToDir(ctx, fcOutputDir, metadata.FileList.Files)
	if err != nil {
		return nil, err
	}

	switch *orderForm {
	case OrderFormRawMaterial:
		err = workflow.ExecuteChildWorkflow(ctx, RawMaterial, RawMaterialParams{
			Metadata:  metadata,
			Directory: fcOutputDir,
		}).Get(ctx, nil)
	case OrderFormVBMaster:
		err = workflow.ExecuteChildWorkflow(ctx, VBMaster, MasterParams{
			Metadata:  metadata,
			OrderForm: *orderForm,
			Directory: fcOutputDir,
		}).Get(ctx, nil)
	case OrderFormSeriesMaster:
		err = workflow.ExecuteChildWorkflow(ctx, SeriesMaster, MasterParams{
			Metadata:  metadata,
			OrderForm: *orderForm,
			Directory: fcOutputDir,
		}).Get(ctx, nil)
	}
	if err != nil {
		return nil, err
	}

	return &AssetResult{}, nil
}

func copyToDir(ctx workflow.Context, dest string, files []ingest.File) error {
	var dirs []string
	for _, file := range files {
		if !lo.Contains(dirs, file.FilePath) {
			dirs = append(dirs, file.FilePath)
		}
	}

	if len(dirs) > 1 {
		return fmt.Errorf("multiple directories not supported: %s", dirs)
	}

	dir, err := utils.ParsePath(filepath.Join("/mnt/dmzshare", "workflow", dirs[0]))
	if err != nil {
		return err
	}

	destPath, err := utils.ParsePath(dest)
	if err != nil {
		return err
	}

	err = workflow.ExecuteActivity(ctx, activities.RcloneCopyDir, activities.RcloneCopyDirInput{
		Source:      dir.RclonePath(),
		Destination: destPath.RclonePath(),
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	for _, file := range files {
		err = wfutils.DeletePath(
			ctx,
			filepath.Join("/mnt/dmzshare", "workflow", file.FilePath, file.FileName),
		)
		if err != nil {
			return err
		}
	}

	return nil
}
