package activities

import (
	"context"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/emails"
)

func (ua UtilActivities) SendEmail(_ context.Context, msg emails.Message) (any, error) {
	var errors []error
	for _, email := range msg.To {
		err := emails.Send(email, msg.Subject, msg.HTML)
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		errMsg := ""
		for _, err := range errors {
			errMsg += err.Error() + "\n"
		}
		return nil, fmt.Errorf("failed to send email: %s", errMsg)
	}

	return nil, nil
}
