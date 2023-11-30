package wfutils

import (
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/services/notifications"
	"go.temporal.io/sdk/workflow"
)

func Notify(ctx workflow.Context, targets []notifications.Target, title, message string) error {
	return ExecuteWithQueue(ctx, activities.NotifyTargets, activities.NotifyTargetsInput{
		Targets: targets,
		Message: notifications.SimpleNotification{
			Title:   title,
			Message: message,
		},
	}).Get(ctx, nil)
}
