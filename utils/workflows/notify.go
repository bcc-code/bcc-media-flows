package wfutils

import (
	"os"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"go.temporal.io/sdk/workflow"
)

func Notify(ctx workflow.Context, targets []notifications.Target, title, message string) (*notifications.SendResult, error) {
	return Execute(ctx, activities.Util.NotifySimple, activities.NotifySimpleInput{
		Targets: targets,
		Message: notifications.SimpleNotification{
			Title:   title,
			Message: message,
		},
	}).Result(ctx)
}

type UpdateTelegramMessageInput struct {
}

func UpdateTelegramMessage(ctx workflow.Context, original *notifications.SendResult, newMessage string) (*notifications.SendResult, error) {
	return Execute(ctx, activities.Util.UpdateTelegramMessage, activities.UpdateTelegramMessageInput{
		OriginalMessage: original.TelegramMessage,
		NewMessage: notifications.SimpleNotification{
			Message: newMessage,
		},
	}).Result(ctx)
}

func NotifyTelegramChannel(ctx workflow.Context, message string) (*notifications.SendResult, error) {
	return Execute(ctx, activities.Util.NotifySimple, activities.NotifySimpleInput{
		Targets: []notifications.Target{
			{
				ID:   os.Getenv("TELEGRAM_CHAT_ID"),
				Type: notifications.TargetTypeTelegram,
			},
		},
		Message: notifications.SimpleNotification{
			Message: message,
		},
	}).Result(ctx)
}
