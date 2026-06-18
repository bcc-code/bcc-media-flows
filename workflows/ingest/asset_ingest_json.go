package ingestworkflows

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ingest"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
)

// sidecarStorageRoot is the absolute root that a relative JSONPath is resolved
// against. The upload portal delivers paths relative to this mount.
const sidecarStorageRoot = "/mnt/filecatalyst/delivery2"

// AssetJSONParams drives the JSON-based ingest path. JSONPath points at the
// JSON sidecar produced by the upload portal; it may be relative (resolved
// against sidecarStorageRoot) or absolute. The media file it describes
// (JSONForm.Filename) sits in the same directory.
type AssetJSONParams struct {
	JSONPath string
}

// AssetJSON is the JSON counterpart to Asset. It parses a JSON order form,
// translates it into the existing ingest.Metadata / OrderForm model and
// dispatches to the same child workflows as the XML pipeline.
func AssetJSON(ctx workflow.Context, params AssetJSONParams) (*AssetResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting AssetJSON")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	jsonPathStr := params.JSONPath
	if !filepath.IsAbs(jsonPathStr) {
		jsonPathStr = filepath.Join(sidecarStorageRoot, jsonPathStr)
	}
	jsonPath := paths.MustParse(jsonPathStr)

	form, err := wfutils.UnmarshalJSONFile[ingest.JSONForm](ctx, jsonPath)
	if err != nil {
		return nil, err
	}

	metadata, orderForm, err := translateJSONForm(*form)
	if err != nil {
		return nil, err
	}

	// The media file sits next to the JSON sidecar.
	mediaPath := jsonPath.Dir().Append(form.Filename)

	targets := lo.Map(strings.Split(metadata.JobProperty.SenderEmail, ","), func(s string, _ int) string {
		return strings.TrimSpace(s)
	})

	wfutils.SendEmails(ctx, targets, "Import triggered", "Order form: "+metadata.JobProperty.OrderForm)

	switch orderForm {
	case OrderFormVBMaster, OrderFormLEDMaterial:
		outputDir, err := wfutils.GetWorkflowMastersOutputFolder(ctx)
		if err != nil {
			return nil, err
		}
		err = workflow.ExecuteChildWorkflow(ctx, Masters, MasterParams{
			Targets:              targets,
			Metadata:             metadata,
			OrderForm:            orderForm,
			SourceFile:           &mediaPath,
			OutputDir:            outputDir,
			KeepOriginalFilename: true,
		}).Get(ctx, nil)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported order form for JSON ingest: %s", orderForm.Value)
	}

	return &AssetResult{}, nil
}
