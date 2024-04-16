package telegram

import (
	"os"
	"time"

	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"gopkg.in/telebot.v3"
)

var (
	telegramBot *telebot.Bot
)

type Notification struct {
	Chat            Chat
	Message         notifications.Template
	telegramMessage *telebot.Message
}

func getOrInitTelegramBot() (*telebot.Bot, error) {
	if telegramBot == nil {
		pref := telebot.Settings{
			Token:  os.Getenv("TELEGRAM_BOT_TOKEN"),
			Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
		}

		bot, err := telebot.NewBot(pref)
		if err != nil {
			return nil, err
		}

		telegramBot = bot
	}

	return telegramBot, nil
}

func SendNotification(notification *Notification) (*Notification, error) {
	bot, err := getOrInitTelegramBot()
	if err != nil {
		return notification, err
	}

	markdown, err := notification.Message.RenderMarkdown()
	if err != nil {
		return notification, err
	}

	var msg *telebot.Message
	if notification.telegramMessage == nil {
		msg, err = bot.Send(
			&telebot.Chat{ID: Chats.Value(notification.Chat)},
			markdown,
			telebot.ModeMarkdown,
		)
	} else {
		msg, err = bot.Edit(
			notification.telegramMessage,
			markdown,
			telebot.ModeMarkdown,
		)
	}

	notification.telegramMessage = msg
	return notification, err
}
