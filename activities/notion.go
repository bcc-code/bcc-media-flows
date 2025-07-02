package activities

import (
	"context"
	"github.com/bcc-code/bcc-media-flows/services/notion"
)

var Notion *NotionActivities

type NotionActivities struct {
	Client *notion.Client
}

// QueryDatabase queries a database in Notion
//
// Filtering is not implemented yet
func (a *NotionActivities) QueryDatabase(ctx context.Context, databaseID string) ([]map[string]interface{}, error) {
	return a.Client.QueryDatabase(databaseID)
}
