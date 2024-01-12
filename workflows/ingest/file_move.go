package ingestworkflows

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ingest"
	"github.com/bcc-code/bcc-media-flows/utils"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type MoveUploadedFilesParams struct {
	OrderForm OrderForm
	Metadata  *ingest.Metadata
	Directory paths.Path

	// Relative to isilon root, We can extend this if we need to have a full path
	OutputDir paths.Path
}

func MoveUploadedFiles(ctx workflow.Context, params MoveUploadedFilesParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting MoveUploadedFiles workflow")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	originalFiles, err := wfutils.ListFiles(ctx, params.Directory)
	if err != nil {
		return err
	}

	errors := []error{}
	for _, f := range originalFiles {
		if !utils.ValidRawFilename(f.Local()) {
			errors = append(errors, fmt.Errorf("invalid filename: %s", f))
			continue
		}

		filename, err := getOrderFormFilename(params.OrderForm, f, params.Metadata.JobProperty)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		newPath := params.OutputDir.Append(filename + f.Ext())

		err = wfutils.MoveFile(ctx, f, newPath)
		if err != nil {
			errors = append(errors, err)
			continue
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors: %v", errors)
	}
	return nil
}
