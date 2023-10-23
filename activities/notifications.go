package activities

import (
	"context"
	"github.com/bcc-code/bccm-flows/services/notifications"
	"go.temporal.io/sdk/activity"
)

type NotifyTargetsInput struct {
	Targets []notifications.Target
	Message string
}

func NotifyTargets(ctx context.Context, input NotifyTargetsInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending notification")

	client := notifications.NewClient(notificationServices{})
	return client.Send(input.Targets, input.Message)
}

// TODO: Implement notification services
type notificationServices struct {
}

func (ns notificationServices) SendEmail(email string, message string) error {
	return nil
}

func (ns notificationServices) SendTelegramMessage(chatID string, message string) error {
	return nil
}

func (ns notificationServices) SendSMS(phoneNumber string, message string) error {
	return nil
}
