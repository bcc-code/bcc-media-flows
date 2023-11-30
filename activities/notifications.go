package activities

import (
	"context"
	"os"

	"github.com/bcc-code/bccm-flows/services/notifications"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"go.temporal.io/sdk/activity"
)

type NotifySimpleInput struct {
	Targets []notifications.Target
	Message notifications.SimpleNotification
}

func NotifySimple(ctx context.Context, input NotifySimpleInput) error {
	logger := activity.GetLogger(ctx)
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
	client := sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
	from := mail.NewEmail("Workflows", "workflows@bcc.media")
	to := mail.NewEmail(email, email)
	var subject string
	content, err := message.RenderHTML()
	if err != nil {
		return err
	}
	switch t := message.(type) {
	case notifications.ImportCompleted:
		subject = t.Title
	case notifications.SimpleNotification:
		subject = t.Title
	}
	m := mail.NewV3MailInit(from, subject, to, mail.NewContent("text/html", content))
	_, err = client.Send(m)
	return err
}

func (ns notificationServices) SendTelegramMessage(chatID string, message notifications.Template) error {
	return nil
}

func (ns notificationServices) SendSMS(phoneNumber string, message notifications.Template) error {
	return nil
}
