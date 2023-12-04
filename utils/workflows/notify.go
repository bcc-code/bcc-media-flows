package wfutils

import (
	"os"

	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/services/notifications"
	"go.temporal.io/sdk/workflow"
)

func Notify(ctx workflow.Context, targets []notifications.Target, title, message string) error {
	return ExecuteWithQueue(ctx, activities.NotifySimple, activities.NotifySimpleInput{
		Targets: targets,
		Message: notifications.SimpleNotification{
			Title:   title,
			Message: message,
		},
	}).Get(ctx, nil)
}

func NotifyTelegramChannel(ctx workflow.Context, message string) error {
	return ExecuteWithQueue(ctx, activities.NotifySimple, activities.NotifySimpleInput{
		Targets: []notifications.Target{
			{
				ID:   os.Getenv("TELEGRAM_CHAT_ID"),
				Type: notifications.TargetTypeTelegram,
			},
		},
		Message: notifications.SimpleNotification{
			Message: message,
		},
	}).Get(ctx, nil)
}
