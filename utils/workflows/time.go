package wfutils

import (
	"go.temporal.io/sdk/workflow"
	"time"
)

func Now(ctx workflow.Context) time.Time {
	se := workflow.SideEffect(ctx, func(ctx workflow.Context) any {
		return time.Now()
	})

	var date time.Time
	se.Get(&date)
	return date
}
