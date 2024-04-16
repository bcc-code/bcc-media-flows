package activities

import (
	"context"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
)

func (ua UtilActivities) SendTelegramMessage(_ context.Context, msg *telegram.Notification) (*telegram.Notification, error) {
	return telegram.SendNotification(msg)
}
