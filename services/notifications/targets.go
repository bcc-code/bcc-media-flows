package notifications

import "github.com/orsinium-labs/enum"

type TargetType enum.Member[string]

var (
	TargetTypeEmail    = TargetType{Value: "email"}
	TargetTypeTelegram = TargetType{Value: "telegram"}
	TargetTypeSMS      = TargetType{Value: "sms"}
	TargetTypes        = []TargetType{
		TargetTypeEmail,
		TargetTypeTelegram,
		TargetTypeSMS,
	}
)

type Target struct {
	// TargetType defines the type of the target, e.g. email, telegram, etc.
	Type TargetType
	// ID can be an email address, a phone number, a telegram chat id, etc.
	ID string
}
