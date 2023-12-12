package activities

import (
	"context"
	"os"

	"github.com/bcc-code/bccm-flows/services/emails"
	"github.com/bcc-code/bccm-flows/services/notifications"
	"github.com/bcc-code/bccm-flows/services/telegram"
	"go.temporal.io/sdk/activity"
)

type NotifySimpleInput struct {
	Targets []notifications.Target
	Message notifications.SimpleNotification
}

func NotifySimple(ctx context.Context, input NotifySimpleInput) error {
	logger := activity.GetLogger(ctx)
	if os.Getenv("DEBUG") != "" && os.Getenv("TELEGRAM_CHAT_ID") == "" {
		logger.Info("Ignoring notification for debug without TELEGRAM_CHAT_ID")
		return nil
	}
	logger.Info("Sending notification")

	client := notifications.NewClient(notificationServices{})
	return client.Send(input.Targets, input.Message)
}

type NotifyImportCompletedInput struct {
	Targets []notifications.Target
	Message notifications.ImportCompleted
}

func NotifyImportCompleted(ctx context.Context, input NotifyImportCompletedInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending notification")

	client := notifications.NewClient(notificationServices{})
	return client.Send(input.Targets, input.Message)
}

type notificationServices struct {
}

func (ns notificationServices) SendEmail(email string, message notifications.Template) error {
	return emails.Send(email, message)
}

func (ns notificationServices) SendTelegramMessage(chatID int64, message notifications.Template) error {
	return telegram.SendTelegramMessage(chatID, message)
}

func (ns notificationServices) SendSMS(phoneNumber string, message notifications.Template) error {
	return nil
}
