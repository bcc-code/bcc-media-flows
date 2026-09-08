package telegram

import (
	"errors"
	"log"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/services/notifications"

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

func (m *Message) UpdateWithTemplate(template notifications.Template) error {
	markdown, err := template.RenderMarkdown()
	if err != nil {
		return err
	}

	m.Markdown = markdown
	return nil
}

func getOrInitTelegramBot() (*telebot.Bot, error) {
	if telegramBot == nil {
		pref := telebot.Settings{
			Token:  environment.Get().Telegram.BotToken(),
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

func Send(message *Message) (*Message, error) {
	bot, err := getOrInitTelegramBot()
	if err != nil {
		return message, err
	}

	msg, err := deliver(bot, message, telebot.ModeMarkdown)
	if IsMarkdownParseError(err) {
		// Telegram rejects the whole message rather than dropping the
		// formatting it cannot parse, so an unbalanced `_` in a filename
		// forwarded from another system would otherwise lose the notification
		// entirely. Delivering it unformatted keeps the alert actionable.
		log.Printf("telegram: markdown rejected (%v), resending as plain text", err)
		msg, err = deliver(bot, message, telebot.ModeDefault)
	}

	message.TelegramMessage = msg
	return message, err
}

// deliver posts a new message or edits the one already sent, with the given
// parse mode. telebot omits parse_mode entirely for ModeDefault, which is how
// the plain-text retry is expressed.
func deliver(bot *telebot.Bot, message *Message, mode telebot.ParseMode) (*telebot.Message, error) {
	if message.TelegramMessage == nil {
		return bot.Send(
			&telebot.Chat{ID: Chats.Value(message.Chat)},
			message.Markdown,
			mode,
		)
	}
	return bot.Edit(
		message.TelegramMessage,
		message.Markdown,
		mode,
	)
}

// IsMarkdownParseError reports whether Telegram refused a message because its
// markup did not parse ("Bad Request: can't parse entities: ..."), as opposed
// to any other API failure. The description carries a byte offset, so only the
// stable part of it can be matched.
func IsMarkdownParseError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *telebot.Error
	if errors.As(err, &apiErr) {
		return strings.Contains(apiErr.Description, "can't parse entities")
	}
	return strings.Contains(err.Error(), "can't parse entities")
}

func SendText(chat Chat, text string) (*Message, error) {
	return Send(&Message{
		Chat:     chat,
		Markdown: text,
	})
}
