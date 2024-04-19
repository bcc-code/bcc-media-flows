package telegram

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
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
