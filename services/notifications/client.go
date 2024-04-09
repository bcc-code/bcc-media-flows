package notifications

import "gopkg.in/telebot.v3"

type Services interface {
	SendEmail(email string, message Template) error
	SendTelegramMessage(chatID int64, message Template) (*telebot.Message, error)
	EditTelegramMessage(msg *telebot.Message, message Template) (*telebot.Message, error)
	SendSMS(phoneNumber string, message Template) error
}

type Client struct {
	services Services
}

func NewClient(services Services) *Client {
	return &Client{
		services: services,
	}
}
