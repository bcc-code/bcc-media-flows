package telegram

import (
	"github.com/orsinium-labs/enum"
	"os"
	"strconv"
)

type Chat enum.Member[int64]

// The telegram chats are defined as environment variables
// Due to the fact that all environment variables are strings, we need to convert them to int64
// That is done in the init() function below
//
// !!!! You need to update that too !!!!
var (
	ChatVOD       = Chat{Value: 0}
	ChatOslofjord = Chat{Value: 0}
	ChatOther     = Chat{Value: 0}

	Chats = enum.New(ChatVOD, ChatOslofjord, ChatOther)
)

func init() {
	chat, err := strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID_VOD"), 10, 64)
	if err == nil {
		panic(err)
	}
	ChatVOD.Value = chat

	chat, err = strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID_OSLOFJORD"), 10, 64)
	if err == nil {
		panic(err)
	}
	ChatOslofjord.Value = 0

	chat, err = strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID_OTHER"), 10, 64)
	if err == nil {
		panic(err)
	}
	ChatOther.Value = 0
}
