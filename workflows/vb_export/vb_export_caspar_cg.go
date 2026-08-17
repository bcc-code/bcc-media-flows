package vb_export

import (
	"github.com/bcc-code/bcc-media-flows/paths"
	"go.temporal.io/sdk/workflow"
)

// VBExportToCasparCG copies the input file directly to the CasparCG delivery folder without transcoding.
func VBExportToCasparCG(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	return runVBExportChild(ctx, params, vbExportDestination{
		destination: DestinationCasparCG,
		copySource:  func(p VBExportChildWorkflowParams) paths.Path { return p.OriginalFile },
	})
}
