package activities

import (
	"context"
	"github.com/bcc-code/bcc-media-flows/services/notion"
)

var Notion *NotionActivities

type NotionActivities struct {
	Client *notion.Client
}

type QueryDatabaseArgs struct {
	DatabaseID string
	Filter     string
}

// QueryDatabase queries a database in Notion
//
// Filtering is not implemented yet
func (a *NotionActivities) QueryDatabase(ctx context.Context, args QueryDatabaseArgs) ([]map[string]interface{}, error) {
	return a.Client.QueryDatabase(args.DatabaseID, args.Filter)
}
