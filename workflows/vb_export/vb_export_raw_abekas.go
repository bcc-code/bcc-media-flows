package vb_export

import (
	"github.com/bcc-code/bcc-media-flows/paths"
	"go.temporal.io/sdk/workflow"
)

// VBExportToRawAbekas copies the input file directly to Abekas-RAW without transcoding.
func VBExportToRawAbekas(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	return runVBExportChild(ctx, params, vbExportDestination{
		destination: DestinationRawAbekas,
		copySource:  func(p VBExportChildWorkflowParams) paths.Path { return p.InputFile },
	})
}
