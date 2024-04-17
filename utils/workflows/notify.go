package wfutils

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"go.temporal.io/sdk/workflow"
)

func NotifyTelegramChannel(ctx workflow.Context, channel telegram.Chat, message string) {
	err := Execute(ctx, activities.Util.SendTelegramMessage, &telegram.Message{
		Message: &notifications.SimpleNotification{
			Message: message,
		},
		Chat: channel,
	}).Get(ctx, nil)

	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to send telegram message", "error", err)
	}
}

func NotifyEmails(ctx workflow.Context, targets []string, subject, message string) {
	err := Execute(ctx, activities.Util.SendEmail, activities.EmailMessageInput{
		Message: &notifications.SimpleNotification{
			Title:   subject,
			Message: message,
		},
		To: targets,
	}).Get(ctx, nil)

	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to send email", "error", err)
	}
}
