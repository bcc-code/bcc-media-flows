package activities

import (
	"context"

	"github.com/bcc-code/bcc-media-flows/services/clickup"
)

var ClickUp *ClickUpActivities

type ClickUpActivities struct {
	Client *clickup.Client
}

type QueryShortsArgs struct{}

// QueryShorts returns every task in the configured public shorts view. Filtering
// to Editorial status "Ready in Mediabanken" and Asset status != "Done" is done
// downstream in mapAndFilterShortsData (the public view can't take arbitrary
// server-side filters).
func (a *ClickUpActivities) QueryShorts(_ context.Context, _ QueryShortsArgs) ([]clickup.Task, error) {
	return a.Client.ListTasks()
}
