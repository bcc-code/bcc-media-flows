package wfutils

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/services/emails"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"go.temporal.io/sdk/workflow"
)

// SendTelegramError reports a failure to the given chat.
//
// vxid is optional; omit it for failures that happen before an asset exists.
func SendTelegramError(ctx workflow.Context, channel telegram.Chat, vxid string, err error) {
	subject := workflow.GetInfo(ctx).WorkflowType.Name
	if vxid != "" {
		subject += fmt.Sprintf(" of `%s`", vxid)
	}

	SendTelegramText(ctx, channel, fmt.Sprintf("🟥 %s failed:\n```\n%s\n```", subject, err.Error()))
}

func SendTelegramText(ctx workflow.Context, channel telegram.Chat, message string) {
	msg, err := telegram.NewMessage(channel, notifications.Simple{Message: message})
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to create telegram message", "error", err)
		return
	}

	err = Execute(ctx, activities.Util.SendTelegramMessage, msg).Wait(ctx)

	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to send telegram message", "error", err)
	}
}

// SendTelegramMessage sends an already-built message. The destination chat is
// carried on msg itself, set when it was created with telegram.NewMessage, so no
// channel argument is taken — an earlier signature had one and silently ignored it.
func SendTelegramMessage(ctx workflow.Context, msg *telegram.Message) *telegram.Message {
	msg, err := Execute(ctx, activities.Util.SendTelegramMessage, msg).Result(ctx)

	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to send telegram message", "error", err)
	}

	return msg
}

func SendEmails(ctx workflow.Context, targets []string, subject, message string) {
	SendEmailTemplate(ctx, targets, notifications.Simple{
		Title:   subject,
		Message: message,
	})
}

// SendEmailTemplate sends a rendered notification template (HTML + plain text)
// to the given recipients.
func SendEmailTemplate(ctx workflow.Context, targets []string, content notifications.Template) {
	msg, err := emails.NewMessage(content, targets, nil, nil)

	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to create email message", "error", err)
		return
	}

	err = Execute(ctx, activities.Util.SendEmail, msg).Wait(ctx)

	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to send email", "error", err)
	}
}
