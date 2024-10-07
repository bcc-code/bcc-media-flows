package emails

import (
	"fmt"
	"os"

	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

func Send(email string, subject string, messageHTML string) error {
	client := sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
	from := mail.NewEmail("Workflows", "workflows@em5370.brunstad.tv")
	to := mail.NewEmail(email, email)

	m := mail.NewV3MailInit(from, subject, to, mail.NewContent("text/html", messageHTML))
	res, err := client.Send(m)
	if err != nil {
		return err
	}

	if res.StatusCode != 202 {
		return fmt.Errorf("failed to send email: %s", res.Body)
	}

	return nil
}

func NewMessage(template notifications.Template, to []string, cc []string, bcc []string) (Message, error) {
	html, err := template.RenderHTML()
	if err != nil {
		return Message{}, err
	}

	plainText, err := template.RenderMarkdown()
	if err != nil {
		return Message{}, err
	}

	return Message{
		HTML:      html,
		PlainText: plainText,
		Subject:   template.Subject(),
		To:        to,
		CC:        cc,
		BCC:       bcc,
	}, nil
}

type Message struct {
	Subject   string
	HTML      string
	PlainText string
	To        []string
	CC        []string
	BCC       []string
}
