package telegram

import (
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"os"
	"time"

	"gopkg.in/telebot.v3"
)

var (
	telegramBot *telebot.Bot
)

func NewMessage(chat Chat, template notifications.Template) (*Message, error) {
	markdown, err := template.RenderMarkdown()

	return &Message{
		Chat:     chat,
		Markdown: markdown,
	}, err
}

type Message struct {
	Chat            Chat
	Markdown        string
	TelegramMessage *telebot.Message
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

	var msg *telebot.Message
	if message.TelegramMessage == nil {
		msg, err = bot.Send(
			&telebot.Chat{ID: Chats.Value(message.Chat)},
			message.Markdown,
			telebot.ModeMarkdown,
		)
	} else {
		msg, err = bot.Edit(
			message.TelegramMessage,
			message.Markdown,
			telebot.ModeMarkdown,
		)
	}

	message.TelegramMessage = msg
	return message, err
}

func SendText(chat Chat, text string) (*Message, error) {
	return SendMessage(&Message{
		Chat:     chat,
		Markdown: text,
	})
}
