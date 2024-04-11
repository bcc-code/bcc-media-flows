package ingestworkflows

import (
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ingest"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
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
	OrderFormUpload       = OrderForm{Value: "Upload"}
	OrderFormMusic        = OrderForm{Value: "Music"}
	OrderForms            = enum.New(
		OrderFormRawMaterial,
		OrderFormMusic,
		OrderFormUpload,
		OrderFormVBMaster,
		OrderFormSeriesMaster,
		OrderFormOtherMaster,
		OrderFormLEDMaterial,
		OrderFormPodcast,
		OrderFormMultitrackPB,
	)
)

type AssetParams struct {
	XMLPath string
}

type AssetResult struct{}

// sanitizeDuplicatdPath removes duplicated path from the string
func sanitizeDuplicatdPath(s string) string {
	if len(s)%2 != 0 {
		return s // Return string if length is odd
	}

	mid := len(s) / 2
	if s[:mid] == s[mid:] {
		return s[:mid] // Return one part if halves are equal
	}
	return s // Return original string if halves are not equal
}

func Asset(ctx workflow.Context, params AssetParams) (*AssetResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting Asset")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	xmlPath, err := paths.Parse(params.XMLPath)
	if err != nil {
		return nil, err
	}

	metadata, err := wfutils.UnmarshalXMLFile[ingest.Metadata](ctx, xmlPath)
	if err != nil {
		return nil, err
	}

	fixedFiles := []string{}

	// For some reason sometimes the paths in the XML are duplicated
	// (e.g. "abc/def/abc/def" instead of "abc/def") so we need to fix that
	for i, file := range metadata.FileList.Files {
		fixedFiles = append(fixedFiles, sanitizeDuplicatdPath(file.FilePath))
		metadata.FileList.Files[i].FilePath = sanitizeDuplicatdPath(file.FilePath)
	}

	orderForm := OrderForms.Parse(metadata.JobProperty.OrderForm)
	if orderForm == nil {
		return nil, fmt.Errorf("unsupported order form: %s", metadata.JobProperty.OrderForm)
	}
	_, err = wfutils.MoveToFolder(ctx,
		xmlPath,
		xmlPath.Dir().Append("processed"),
		rclone.PriorityNormal,
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
		err = workflow.ExecuteChildWorkflow(ctx, RawMaterialForm, RawMaterialFormParams{
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
	case OrderFormUpload:
		outputDir, err := wfutils.GetWorkflowIsilonOutputFolder(ctx, "Input/FromDelivery")
		if err != nil {
			return nil, err
		}

		err = workflow.ExecuteChildWorkflow(ctx, MoveUploadedFiles, MoveUploadedFilesParams{
			OrderForm: *orderForm,
			Metadata:  metadata,
			Directory: fcOutputDir,
			OutputDir: outputDir,
		}).Get(ctx, nil)
	case OrderFormMusic:
		outputDir := wfutils.GetWorkflowLucidLinkOutputFolder(ctx, "08 From Delivery")
		if err != nil {
			return nil, err
		}

		err = workflow.ExecuteChildWorkflow(ctx, MoveUploadedFiles, MoveUploadedFilesParams{
			OrderForm: *orderForm,
			Metadata:  metadata,
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

	dir, err := paths.Parse(filepath.Join("/mnt/filecatalyst/workflow", dirs[0]))
	if err != nil {
		return err
	}

	err = wfutils.RcloneCopyDir(ctx, dir.Rclone(), dest.Rclone(), rclone.PriorityNormal)
	if err != nil {
		return err
	}

	for _, file := range files {
		err = wfutils.DeletePathRecursively(
			ctx,
			paths.MustParse(filepath.Join("/mnt/filecatalyst/workflow", file.FilePath, file.FileName)),
		)
		if err != nil {
			return err
		}
	}

	return nil
}
