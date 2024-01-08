package telegram

import (
	"os"
	"time"

	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"gopkg.in/telebot.v3"
)

func SendTelegramMessage(chatID int64, message notifications.Template) error {
	pref := telebot.Settings{
		Token:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := telebot.NewBot(pref)
	markdown, err := message.RenderMarkdown()
	if err != nil {
		return err
	}
	_, err = bot.Send(&telebot.Chat{
		ID: chatID,
	}, markdown, telebot.ModeMarkdown)
	if err != nil {
		return err
	}
	return nil
}
