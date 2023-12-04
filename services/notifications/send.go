package notifications

import "strconv"

type Template interface {
	IsTemplate()
	RenderHTML() (string, error)
	RenderMarkdown() (string, error)
}

func (c *Client) Send(targets []Target, message Template) error {

	for _, target := range targets {
		switch target.Type {
		case TargetTypeEmail:
			return c.services.SendEmail(target.ID, message)
		case TargetTypeTelegram:
			intID, err := strconv.ParseInt(target.ID, 10, 64)
			if err != nil {
				return err
			}
			return c.services.SendTelegramMessage(intID, message)
		case TargetTypeSMS:
			return c.services.SendSMS(target.ID, message)
		}
	}

	return nil
}
