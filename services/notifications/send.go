package notifications

func (c *Client) Send(targets []Target, message string) error {

	for _, target := range targets {
		switch target.Type {
		case TargetTypeEmail:
			return c.services.SendEmail(target.ID, message)
		case TargetTypeTelegram:
			return c.services.SendTelegramMessage(target.ID, message)
		case TargetTypeSMS:
			return c.services.SendSMS(target.ID, message)
		}
	}

	return nil
}
