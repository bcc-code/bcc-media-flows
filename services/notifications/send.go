package notifications

import (
	"gopkg.in/telebot.v3"
	"strconv"
)

type Template interface {
	IsTemplate()
	RenderHTML() (string, error)
	RenderMarkdown() (string, error)
}

type SendResult struct {
	TelegramMessage *telebot.Message
}

func (c *Client) Update(msg *telebot.Message, message Template) (*SendResult, error) {
	newMsg, err := c.services.EditTelegramMessage(msg, message)
	return &SendResult{TelegramMessage: newMsg}, err
}

func (c *Client) Send(targets []Target, message Template) (*SendResult, error) {

	for _, target := range targets {
		switch target.Type {
		case TargetTypeEmail:
			return &SendResult{}, c.services.SendEmail(target.ID, message)
		case TargetTypeTelegram:
			intID, err := strconv.ParseInt(target.ID, 10, 64)
			if err != nil {
				return nil, err
			}
			msg, err := c.services.SendTelegramMessage(intID, message)
			return &SendResult{TelegramMessage: msg}, err
		case TargetTypeSMS:
			return &SendResult{}, c.services.SendSMS(target.ID, message)
		}
	}

	return &SendResult{}, nil
}
