package telegram

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMessageJson tests the marshalling and unmarshalling of the Message struct
//
// This may look stupid but the WF SDK uses JSON to serialize the structs, so things break if
// the JSON marshalling/unmarshalling is not working properly.
func TestMessageJson(t *testing.T) {
	message := Message{
		Markdown: "ASDASD",
		Chat:     ChatOslofjord,
	}

	marshalled, err := json.Marshal(message)
	assert.NoError(t, err)

	expected := Message{}
	err = json.Unmarshal(marshalled, &expected)
	assert.NoError(t, err)
	assert.Equal(t, expected, message)
}

// TestInvalidMarkdown tests sending a message with invalid markdown
//
// Telegram should return an error
func TestInvalidMarkdown(t *testing.T) {
	if os.Getenv("TELEGRAM_BOT_TOKEN") == "" {
		t.Skip("TELEGRAM_BOT_TOKEN is not set")
	}

	// Create a new message
	chat := Chat{Value: 0}
	message := Message{
		Markdown: "Something AS22_20221203_2000_SEQ something",
		Chat:     chat,
	}

	// Send the message
	_, err := Send(&message)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "can't parse entities: Can't find end of the entity starting at byte offset")

	message.Markdown = "Something `AS22_20221203_2000_SEQ` something"

	// Send the message
	_, err = Send(&message)
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "can't parse entities: Can't find end of the entity starting at byte offset")
}
