package activities

import (
	"context"
	"github.com/bcc-code/bcc-media-flows/services/notion"
)

var Notion *NotionActivities

type NotionActivities struct {
	Client           *notion.Client
	ShortsDatabaseID string
}

type QueryDatabaseArgs struct {
	DatabaseID string
	Filter     string
}

type UpdateAssetStatusArgs struct {
	RowID  string
	Status string
}

// QueryDatabase queries a database in Notion
func (a *NotionActivities) QueryDatabase(ctx context.Context, args QueryDatabaseArgs) ([]map[string]interface{}, error) {
	return a.Client.QueryDatabase(args.DatabaseID, args.Filter)
}

// UpdateAssetStatus updates the "Asset Status" property for a Notion page (row)
//
// This is very specific to the Shorts export workflow, and should not be used for any other purpose
func (a *NotionActivities) UpdateAssetStatus(ctx context.Context, args UpdateAssetStatusArgs) (any, error) {
	return nil, a.Client.UpdateAssetStatus(args.RowID, args.Status)
}
