package notifications

type Services interface {
	SendEmail(email string, message string) error
	SendTelegramMessage(chatID string, message string) error
	SendSMS(phoneNumber string, message string) error
}

type Client struct {
	services Services
}

func NewClient(services Services) *Client {
	return &Client{
		services: services,
	}
}
