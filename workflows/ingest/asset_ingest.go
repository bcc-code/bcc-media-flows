package ingestworkflows

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/services/ingest"
	"github.com/bcc-code/bccm-flows/services/notifications"
	"github.com/bcc-code/bccm-flows/utils/workflows"
	"github.com/orsinium-labs/enum"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
)

type OrderForm enum.Member[string]

var (
	OrderFormRawMaterial  = OrderForm{Value: "Rawmaterial"}
	OrderFormVBMaster     = OrderForm{Value: "VB"}
	OrderFormSeriesMaster = OrderForm{Value: "Series_Masters"}
	OrderFormOtherMaster  = OrderForm{Value: "Other_Masters"} // TODO: set correct value
	OrderFormLEDMaterial  = OrderForm{Value: "LED-Material"}
	OrderFormPodcast      = OrderForm{Value: "Podcast"}
	OrderFormMultitrackPB = OrderForm{Value: "MultitrackPB"}
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

	xmlPath, err := paths.Parse(params.XMLPath)
	if err != nil {
		return nil, err
	}

	metadata, err := wfutils.UnmarshalXMLFile[ingest.Metadata](ctx, xmlPath)
	if err != nil {
		return nil, err
	}

	orderForm := OrderForms.Parse(metadata.JobProperty.OrderForm)
	if orderForm == nil {
		return nil, fmt.Errorf("unsupported order form: %s", metadata.JobProperty.OrderForm)
	}
	_, err = wfutils.MoveToFolder(ctx,
		xmlPath,
		xmlPath.Dir().Append("processed"),
	)
	if err != nil {
		return nil, err
	}

	tempDir, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	fcOutputDir := tempDir.Append("fc")
	err = wfutils.CreateFolder(ctx, fcOutputDir)
	if err != nil {
		return nil, err
	}

	err = copyToDir(ctx, fcOutputDir, metadata.FileList.Files)
	if err != nil {
		return nil, err
	}

	targets := lo.Map(strings.Split(metadata.JobProperty.SenderEmail, ","), func(s string, _ int) notifications.Target {
		return notifications.Email(strings.TrimSpace(s))
	})

	err = wfutils.Notify(ctx, targets, "Import triggered", "Order form: "+metadata.JobProperty.OrderForm)
	if err != nil {
		return nil, err
	}

	switch *orderForm {
	case OrderFormRawMaterial:
		err = workflow.ExecuteChildWorkflow(ctx, RawMaterial, RawMaterialParams{
			Targets:   targets,
			OrderForm: *orderForm,
			Metadata:  metadata,
			Directory: fcOutputDir,
		}).Get(ctx, nil)
	case OrderFormSeriesMaster, OrderFormOtherMaster, OrderFormVBMaster, OrderFormLEDMaterial, OrderFormPodcast:
		var outputDir paths.Path
		outputDir, err = wfutils.GetWorkflowMastersOutputFolder(ctx)
		if err != nil {
			return nil, err
		}
		err = workflow.ExecuteChildWorkflow(ctx, Masters, MasterParams{
			Targets:   targets,
			Metadata:  metadata,
			OrderForm: *orderForm,
			Directory: fcOutputDir,
			OutputDir: outputDir,
		}).Get(ctx, nil)
	case OrderFormMultitrackPB:
		var outputDir paths.Path
		outputDir, err = wfutils.GetWorkflowRawOutputFolder(ctx)
		if err != nil {
			return nil, err
		}
		err = workflow.ExecuteChildWorkflow(ctx, Multitrack, MasterParams{
			Targets:   targets,
			Metadata:  metadata,
			OrderForm: *orderForm,
			Directory: fcOutputDir,
			OutputDir: outputDir,
		}).Get(ctx, nil)
	}
	if err != nil {
		return nil, err
	}

	return &AssetResult{}, nil
}

func copyToDir(ctx workflow.Context, dest paths.Path, files []ingest.File) error {
	var dirs []string
	for _, file := range files {
		if !lo.Contains(dirs, file.FilePath) {
			dirs = append(dirs, file.FilePath)
		}
	}

	if len(dirs) > 1 {
		return fmt.Errorf("multiple directories not supported: %s", dirs)
	}

	dir, err := paths.Parse(filepath.Join("/mnt/dmzshare", "workflow", dirs[0]))
	if err != nil {
		return err
	}

	err = workflow.ExecuteActivity(ctx, activities.RcloneCopyDir, activities.RcloneCopyDirInput{
		Source:      dir.Rclone(),
		Destination: dest.Rclone(),
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	for _, file := range files {
		err = wfutils.DeletePath(
			ctx,
			paths.MustParse(filepath.Join("/mnt/dmzshare", "workflow", file.FilePath, file.FileName)),
		)
		if err != nil {
			return err
		}
	}

	return nil
}
