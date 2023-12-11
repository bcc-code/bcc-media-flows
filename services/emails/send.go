package emails

import (
	"os"

	"github.com/bcc-code/bccm-flows/services/notifications"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

func Send(email string, message notifications.Template) error {
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
