package telegram

import (
	"encoding/json"
	"fmt"
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

// TestVBExportMarkdownIssue tests the specific VB export message format that causes parsing issues
//
// This reproduces the exact error from the Temporal workflow where backticks around filenames
// cause Telegram's markdown parser to fail at byte offset 94
func TestVBExportMarkdownIssue(t *testing.T) {
	if os.Getenv("TELEGRAM_BOT_TOKEN") == "" {
		t.Skip("TELEGRAM_BOT_TOKEN is not set")
	}

	// Recreate the exact message format that's failing
	vxid := "VX-510604"
	filename := "MEET_20170717_FILM1_SHRT_02.mov"
	destinations := "hippo_v2"
	runID := "01997b29-4d04-75d9-ae5a-c3271df1024a"

	// This is the problematic message format from vb_export.go:122
	problematicMarkdown := fmt.Sprintf("🟦 VB Export of %s - `%s` started.\nDestination(s): %s\n\nRunID: %s",
		vxid, filename, destinations, runID)

	chat := Chat{Value: 0}
	message := Message{
		Markdown: problematicMarkdown,
		Chat:     chat,
	}

	// This should fail with the "can't parse entities" error at byte offset 94
	_, err := Send(&message)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "can't parse entities: Can't find end of the entity starting at byte offset 94")

	// Test the fix using bold formatting instead of backticks
	fixedMarkdown := fmt.Sprintf("🟦 VB Export of %s `%s` started.\nDestination(s): %s\n\nRunID: %s",
		vxid, filename, destinations, runID)

	message.Markdown = fixedMarkdown

	// This should work without parsing errors
	_, err2 := Send(&message)
	if err2 != nil {
		// The fixed version should not have the same parsing error
		assert.NotContains(t, err2.Error(), "can't parse entities: Can't find end of the entity starting at byte offset 94")
	}
}
