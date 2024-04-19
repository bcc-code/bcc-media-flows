package activities

import (
	"context"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
)

func (ua UtilActivities) SendTelegramMessage(_ context.Context, msg *telegram.Message) (*telegram.Message, error) {
	return telegram.Send(msg)
}
