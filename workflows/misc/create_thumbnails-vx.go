package miscworkflows

import (
	"time"

	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"

	"go.temporal.io/sdk/workflow"
)

// CreateThumbnailsVXInput is the input to the CreateThumbnailsVX workflow
type CreateThumbnailsVXInput struct {
	VXID   string
	Width  int
	Height int
	Delay  time.Duration
}

// CreateThumbnailsVX is the workflow definition for creating thumbnails for a VX asset.
// Default thumbnail size is 320x180 if width/height are not specified.
func CreateThumbnailsVX(
	ctx workflow.Context,
	params CreateThumbnailsVXInput,
) error {
	if params.Delay > 0 {
		logger := workflow.GetLogger(ctx)
		logger.Info("Delaying workflow execution", "duration", params.Delay)
		workflow.Sleep(ctx, params.Delay)
	}

	logger := workflow.GetLogger(ctx)
	logger.Info("Starting CreateThumbnailsVX")

	// Set default thumbnail dimensions if not specified
	if params.Width == 0 {
		params.Width = 320
		params.Height = 180
	}

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	err := wfutils.Execute(ctx, vsactivity.Activities{}.CreateThumbnailsActivity, vsactivity.CreateThumbnailsParams{
		AssetID: params.VXID,
		Width:   params.Width,
		Height:  params.Height,
	}).Wait(ctx)

	return err
}
