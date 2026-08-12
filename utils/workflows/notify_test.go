package wfutils

import (
	"errors"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type NotifyTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

// The telegram package resolves ChatVOD/ChatBMM/... from TELEGRAM_CHAT_ID_* at
// package init, so without those set every one of them is Chat{Value: 0} and they
// are indistinguishable. Tests therefore use explicit distinct values, so an
// assertion about routing cannot pass vacuously.
var (
	testChatBMM   = telegram.Chat{Value: 111}
	testChatOther = telegram.Chat{Value: 222}
)

type sendErrorInput struct {
	Chat telegram.Chat
	VXID string
	Err  string
}

func sendTelegramErrorWorkflow(ctx workflow.Context, in sendErrorInput) error {
	ctx = workflow.WithActivityOptions(ctx, GetDefaultActivityOptions())
	SendTelegramError(ctx, in.Chat, in.VXID, errors.New(in.Err))
	return nil
}

// captureMessage mocks the send activity and returns a pointer to what it received.
func (s *NotifyTestSuite) captureMessage(env *testsuite.TestWorkflowEnvironment) **telegram.Message {
	captured := new(*telegram.Message)
	env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.MatchedBy(
		func(msg *telegram.Message) bool {
			*captured = msg
			return true
		},
	)).Return(&telegram.Message{}, nil)
	return captured
}

// The regression: the channel argument was ignored and telegram.ChatOther
// hardcoded, so every BMM ingest failure was reported to the wrong chat.
func (s *NotifyTestSuite) Test_SendTelegramError_UsesTheGivenChannel() {
	env := s.NewTestWorkflowEnvironment()
	captured := s.captureMessage(env)

	env.ExecuteWorkflow(sendTelegramErrorWorkflow, sendErrorInput{
		Chat: testChatBMM,
		VXID: "VX-1",
		Err:  "ingest blew up",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	s.Require().NotNil(*captured, "no telegram message was sent")
	s.Equal(testChatBMM, (*captured).Chat, "must report to the chat the caller asked for")
	s.NotEqual(testChatOther, (*captured).Chat)
	s.Contains((*captured).Markdown, "ingest blew up")
	s.Contains((*captured).Markdown, "VX-1")
}

// A different channel must route differently, so the first test cannot be passing
// by coincidence.
func (s *NotifyTestSuite) Test_SendTelegramError_RoutesPerChannel() {
	env := s.NewTestWorkflowEnvironment()
	captured := s.captureMessage(env)

	env.ExecuteWorkflow(sendTelegramErrorWorkflow, sendErrorInput{
		Chat: testChatOther,
		VXID: "VX-2",
		Err:  "export blew up",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	s.Require().NotNil(*captured)
	s.Equal(testChatOther, (*captured).Chat)
	s.NotEqual(testChatBMM, (*captured).Chat)
}

// Many callers have no asset yet and pass "". The message should not then contain
// an empty quoted identifier.
func (s *NotifyTestSuite) Test_SendTelegramError_OmitsEmptyVXID() {
	env := s.NewTestWorkflowEnvironment()
	captured := s.captureMessage(env)

	env.ExecuteWorkflow(sendTelegramErrorWorkflow, sendErrorInput{
		Chat: testChatBMM,
		VXID: "",
		Err:  "failed before the asset existed",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	s.Require().NotNil(*captured)
	// The old message read "Export of `` failed" when vxid was empty.
	s.NotContains((*captured).Markdown, "of `", "empty vxid should be left out entirely")
	s.Contains((*captured).Markdown, "failed before the asset existed")
	// The workflow type still identifies what failed.
	s.Contains((*captured).Markdown, "sendTelegramErrorWorkflow")
}

func TestNotifyTestSuite(t *testing.T) {
	suite.Run(t, new(NotifyTestSuite))
}
