package telegram

import (
	"os"
	"time"

	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"gopkg.in/telebot.v3"
)

func SendTelegramMessage(chatID int64, message notifications.Template) (*telebot.Message, error) {
	pref := telebot.Settings{
		Token:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := telebot.NewBot(pref)
	markdown, err := message.RenderMarkdown()
	if err != nil {
		return nil, err
	}
	msg, err := bot.Send(&telebot.Chat{
		ID: chatID,
	}, markdown, telebot.ModeMarkdown)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func EditTelegramMessage(msg *telebot.Message, message notifications.Template) (*telebot.Message, error) {
	pref := telebot.Settings{
		Token:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := telebot.NewBot(pref)
	markdown, err := message.RenderMarkdown()
	if err != nil {
		return nil, err
	}

	return bot.Edit(msg, markdown, telebot.ModeMarkdown)
}
