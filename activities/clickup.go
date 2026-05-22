package activities

import (
	"context"

	"github.com/bcc-code/bcc-media-flows/services/clickup"
)

var ClickUp *ClickUpActivities

type ClickUpActivities struct {
	Client       *clickup.Client
	ShortsListID string
}

type QueryShortsArgs struct{}

type UpdateAssetStatusArgs struct {
	TaskID   string
	OptionID string
}

// QueryShorts returns every task in the configured shorts list whose
// Editorial status is "Ready in Mediabanken" and whose Asset status is not "Done".
func (a *ClickUpActivities) QueryShorts(_ context.Context, _ QueryShortsArgs) ([]clickup.Task, error) {
	filters := []clickup.CustomFieldFilter{
		{
			FieldID:  clickup.FieldEditorialStatusID,
			Operator: "=",
			Value:    clickup.OptionEditorialReadyInMediabanken,
		},
		{
			FieldID:  clickup.FieldAssetStatusID,
			Operator: "!=",
			Value:    clickup.OptionAssetStatusDone,
		},
	}
	return a.Client.ListTasks(a.ShortsListID, filters, false)
}

// UpdateAssetStatus sets the "Asset status" drop-down field on a ClickUp task
// to the given option (e.g. clickup.OptionAssetStatusDone).
func (a *ClickUpActivities) UpdateAssetStatus(_ context.Context, args UpdateAssetStatusArgs) (any, error) {
	return nil, a.Client.SetCustomFieldDropDown(args.TaskID, clickup.FieldAssetStatusID, args.OptionID)
}
