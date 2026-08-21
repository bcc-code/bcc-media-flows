package telegram

import (
	"github.com/bcc-code/bcc-media-flows/environment"

	"github.com/orsinium-labs/enum"
)

type Chat enum.Member[int64]

// The telegram chats are defined as environment variables
// Due to the fact that all environment variables are strings, we need to convert them to int64
// That is done in the init() function below
//
// !!!! You need to update that too !!!!
var (
	ChatVOD       Chat
	ChatOslofjord Chat
	ChatOther     Chat
	ChatBMM       Chat

	Chats enum.Enum[Chat, int64]
)

func init() {
	cfg := environment.Get()

	ChatVOD.Value = cfg.Telegram.ChatVOD()
	ChatOslofjord.Value = cfg.Telegram.ChatOslofjord()
	ChatOther.Value = cfg.Telegram.ChatOther()
	ChatBMM.Value = cfg.Telegram.ChatBMM()

	Chats = enum.New(ChatVOD, ChatOslofjord, ChatOther, ChatBMM)
}
