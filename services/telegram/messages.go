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

type Message struct {
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

func SendMessage(message *Message) (*Message, error) {
	bot, err := getOrInitTelegramBot()
	if err != nil {
		return message, err
	}

	markdown, err := message.Message.RenderMarkdown()
	if err != nil {
		return message, err
	}

	var msg *telebot.Message
	if message.telegramMessage == nil {
		msg, err = bot.Send(
			&telebot.Chat{ID: Chats.Value(message.Chat)},
			markdown,
			telebot.ModeMarkdown,
		)
	} else {
		msg, err = bot.Edit(
			message.telegramMessage,
			markdown,
			telebot.ModeMarkdown,
		)
	}

	message.telegramMessage = msg
	return message, err
}
