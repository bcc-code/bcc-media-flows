package miscworkflows

import (
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type CopyFileInput struct {
	Source      string
	Destination string
}

func CopyFile(ctx workflow.Context, params CopyFileInput) error {
	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	source, err := paths.Parse(params.Source)
	if err != nil {
		return err
	}
	destination, err := paths.Parse(params.Destination)
	if err != nil {
		return err
	}

	return wfutils.CopyFile(ctx, source, destination)
}
