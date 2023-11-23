package notifications

type Services interface {
	SendEmail(email string, message Message) error
	SendTelegramMessage(chatID string, message Message) error
	SendSMS(phoneNumber string, message Message) error
}

type Client struct {
	services Services
}

func NewClient(services Services) *Client {
	return &Client{
		services: services,
	}
}
