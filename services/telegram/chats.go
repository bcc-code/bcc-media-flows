package telegram

import (
	"fmt"
	"os"
	"strconv"

	"github.com/orsinium-labs/enum"
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
	if err != nil {
		fmt.Printf("Error parsing TELEGRAM_CHAT_ID_VOD: %v\n", err)
	}
	ChatVOD.Value = chat

	chat, err = strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID_OSLOFJORD"), 10, 64)
	if err != nil {
		fmt.Printf("Error parsing TELEGRAM_CHAT_ID_OSLOFJORD: %v\n", err)
	}
	ChatOslofjord.Value = chat

	chat, err = strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID_OTHER"), 10, 64)
	if err != nil {
		fmt.Printf("Error parsing TELEGRAM_CHAT_ID_OTHER: %v\n", err)
	}
	ChatOther.Value = chat
}
