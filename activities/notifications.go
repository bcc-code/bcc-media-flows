package activities

import (
	"context"
	"os"

	"github.com/bcc-code/bccm-flows/services/notifications"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"go.temporal.io/sdk/activity"
)

type NotifyTargetsInput struct {
	Targets []notifications.Target
	Message notifications.Message
}

func NotifyTargets(ctx context.Context, input NotifyTargetsInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending notification")

	client := notifications.NewClient(notificationServices{})
	return client.Send(input.Targets, input.Message)
}

type notificationServices struct {
}

func (ns notificationServices) SendEmail(email string, message notifications.Message) error {
	client := sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
	from := mail.NewEmail("Workflows", "workflows@bcc.media")
	to := mail.NewEmail(email, email)
	subject := message.Title
	content := mail.NewContent("text/plain", message.Content)
	m := mail.NewV3MailInit(from, subject, to, content)
	_, err := client.Send(m)
	return err
}

func (ns notificationServices) SendTelegramMessage(chatID string, message notifications.Message) error {
	return nil
}

func (ns notificationServices) SendSMS(phoneNumber string, message notifications.Message) error {
	return nil
}
