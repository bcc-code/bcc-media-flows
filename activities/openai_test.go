package activities

import (
	"context"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_AskChatGpt(t *testing.T) {

	t.Skip("ChatGPT tests can only be run manually")

	u := UtilActivities{}

	ctx := context.Background()
	r, err := u.AskChatGPT(ctx, AskChatGPTInput{})
	assert.NoError(t, err)

	spew.Dump(r)

}
