package wfutils

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/services/emails"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"go.temporal.io/sdk/workflow"
)

func NotifyTelegramChannel(ctx workflow.Context, channel telegram.Chat, message string) {
	msg, err := telegram.NewMessage(channel, notifications.SimpleNotification{
		Message: message,
	})

	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to create telegram message", "error", err)
		return
	}

	err = Execute(ctx, activities.Util.SendTelegramMessage, msg).Get(ctx, nil)

	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to send telegram message", "error", err)
	}
}

func NotifyEmails(ctx workflow.Context, targets []string, subject, message string) {
	msg, err := emails.NewMessage(notifications.SimpleNotification{
		Title:   subject,
		Message: message,
	}, targets, nil, nil)

	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to create email message", "error", err)
		return
	}

	err = Execute(ctx, activities.Util.SendEmail, msg).Get(ctx, nil)

	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to send email", "error", err)
	}
}
