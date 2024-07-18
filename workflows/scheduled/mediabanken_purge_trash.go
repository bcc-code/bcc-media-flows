package scheduled

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type MediabankenPurgeTrashResult struct {
	DeletedVXIDs []string
}

func MediabankenPurgeTrash(ctx workflow.Context) (*MediabankenPurgeTrashResult, error) {
	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	trashedIDs, err := wfutils.Execute(ctx, activities.Vidispine.GetTrashedItems, nil).Result(ctx)
	if err != nil {
		return nil, err
	}

	err = wfutils.Execute(ctx, activities.Vidispine.DeleteItems, vsactivity.DeleteItemsParams{
		VXIDs: trashedIDs,
	}).Wait(ctx)

	if err != nil {
		return nil, err
	}

	return &MediabankenPurgeTrashResult{
		DeletedVXIDs: trashedIDs,
	}, nil
}
