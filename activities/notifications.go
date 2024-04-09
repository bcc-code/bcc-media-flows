package activities

import (
	"context"
	"gopkg.in/telebot.v3"
	"os"

	"github.com/bcc-code/bcc-media-flows/services/emails"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"go.temporal.io/sdk/activity"
)

type NotifySimpleInput struct {
	Targets []notifications.Target
	Message notifications.SimpleNotification
}

func (ua UtilActivities) NotifyTelegramChannel(ctx context.Context, message string) (*notifications.SendResult, error) {
	return ua.NotifySimple(ctx, NotifySimpleInput{
		Targets: []notifications.Target{
			{
				ID:   os.Getenv("TELEGRAM_CHAT_ID"),
				Type: notifications.TargetTypeTelegram,
			},
		},
		Message: notifications.SimpleNotification{
			Message: message,
		},
	})
}

type UpdateTelegramMessageInput struct {
	OriginalMessage *telebot.Message
	NewMessage      notifications.SimpleNotification
}

func (ua UtilActivities) updateTelegramMessage(ctx context.Context, original *telebot.Message, new string) (*notifications.SendResult, error) {
	return ua.UpdateTelegramMessage(ctx, UpdateTelegramMessageInput{
		OriginalMessage: original,
		NewMessage: notifications.SimpleNotification{
			Message: new,
		},
	})
}

func (ua UtilActivities) UpdateTelegramMessage(ctx context.Context, input UpdateTelegramMessageInput) (*notifications.SendResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating Telegram message")

	client := notifications.NewClient(notificationServices{})
	return client.Update(input.OriginalMessage, input.NewMessage)
}

func (ua UtilActivities) NotifySimple(ctx context.Context, input NotifySimpleInput) (*notifications.SendResult, error) {
	logger := activity.GetLogger(ctx)
	if os.Getenv("DEBUG") != "" && os.Getenv("TELEGRAM_CHAT_ID") == "" {
		logger.Info("Ignoring notification for debug without TELEGRAM_CHAT_ID")
		return nil, nil
	}
	logger.Info("Sending notification")

	client := notifications.NewClient(notificationServices{})
	return client.Send(input.Targets, input.Message)
}

type NotifyImportCompletedInput struct {
	Targets []notifications.Target
	Message notifications.ImportCompleted
}

func (ua UtilActivities) NotifyImportCompleted(ctx context.Context, input NotifyImportCompletedInput) (any, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending notification")

	client := notifications.NewClient(notificationServices{})
	_, err := client.Send(input.Targets, input.Message)
	return nil, err
}

type NotifyImportFailedInput struct {
	Targets []notifications.Target
	Message notifications.ImportFailed
}

func (ua UtilActivities) NotifyImportFailed(ctx context.Context, input NotifyImportFailedInput) (any, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending notification")

	client := notifications.NewClient(notificationServices{})
	_, err := client.Send(input.Targets, input.Message)
	return nil, err
}

type notificationServices struct {
}

func (ns notificationServices) SendEmail(email string, message notifications.Template) error {
	return emails.Send(email, message)
}

func (ns notificationServices) SendTelegramMessage(chatID int64, message notifications.Template) (*telebot.Message, error) {
	return telegram.SendTelegramMessage(chatID, message)
}

func (ns notificationServices) EditTelegramMessage(msg *telebot.Message, message notifications.Template) (*telebot.Message, error) {
	return telegram.EditTelegramMessage(msg, message)
}

func (ns notificationServices) SendSMS(phoneNumber string, message notifications.Template) error {
	return nil
}
