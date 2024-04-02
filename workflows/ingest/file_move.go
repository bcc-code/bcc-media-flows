package ingestworkflows

import (
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/rclone"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ingest"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type MoveUploadedFilesParams struct {
	OrderForm OrderForm
	Metadata  *ingest.Metadata
	Directory paths.Path
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

	var errors []error
	for _, f := range originalFiles {
		filename, err := getOrderFormFilename(params.OrderForm, f, params.Metadata.JobProperty)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		newPath := params.OutputDir.Append(filename + f.Ext())

		err = wfutils.MoveFile(ctx, f, newPath, rclone.PriorityNormal)
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
