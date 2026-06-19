package activities

import (
	"context"
	"errors"
	"strings"

	"github.com/bcc-code/bcc-media-flows/services/emails"
)

func (ua UtilActivities) SendEmail(_ context.Context, msg emails.Message) (any, error) {
	var errs []error
	for _, email := range msg.To {
		// Skip blank recipients. Recipient lists are built by splitting a
		// comma-separated field, so trailing commas or an empty sender field
		// yield empty entries that SendGrid rejects outright.
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}

		if err := emails.Send(email, msg.Subject, msg.PlainText, msg.HTML); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return nil, nil
}
