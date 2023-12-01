package activities

import (
	"context"
	"os"

	"github.com/bcc-code/bccm-flows/services/notifications"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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

func (ns notificationServices) SendTelegramMessage(channelID string, message notifications.Template) error {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		return err
	}
	markdown, err := message.RenderMarkdown()
	if err != nil {
		return err
	}
	msg := tgbotapi.NewMessageToChannel(channelID, markdown)
	msg.ParseMode = tgbotapi.ModeMarkdown
	_, err = bot.Send(msg)
	if err != nil {
		return err
	}
	return nil
}

func (ns notificationServices) SendSMS(phoneNumber string, message notifications.Template) error {
	return nil
}
