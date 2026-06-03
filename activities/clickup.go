package activities

import (
	"context"

	"github.com/bcc-code/bcc-media-flows/services/clickup"
)

var ClickUp *ClickUpActivities

type ClickUpActivities struct {
	Client *clickup.Client
}

// QueryShorts returns every task in the configured public shorts view. Filtering
// to Editorial status "Ready in Mediabanken" and Asset status != "Done" is done
// downstream in mapAndFilterShortsData (the public view can't take arbitrary
// server-side filters). It takes no input; the unused arg satisfies the
// wfutils.Execute (ctx, params) activity signature.
func (a *ClickUpActivities) QueryShorts(_ context.Context, _ any) ([]clickup.Task, error) {
	return a.Client.ListTasks()
}
