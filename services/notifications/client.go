package notifications

type Services interface {
	SendEmail(email string, message Template) error
	SendTelegramMessage(chatID int64, message Template) error
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
