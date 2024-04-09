package activities

import (
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"testing"
	"time"
)

type UnitTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestActivityEnvironment
}

func (s *UnitTestSuite) SetupTest() {
	s.env = s.NewTestActivityEnvironment()
}

func (s *UnitTestSuite) TestUpdateMessage() {
	s.env.RegisterActivity(Util.NotifyTelegramChannel)
	s.env.RegisterActivity(Util.UpdateTelegramMessage)

	res, err := s.env.ExecuteActivity(Util.NotifyTelegramChannel, "Hello, World!")
	assert.NoErrorf(s.T(), err, "Error sending notification: %v", err)
	r := &notifications.SendResult{}
	res.Get(r)

	time.Sleep(10 * time.Second)

	res, err = s.env.ExecuteActivity(Util.UpdateTelegramMessage,
		UpdateTelegramMessageInput{
			OriginalMessage: r.TelegramMessage,
			NewMessage: notifications.SimpleNotification{
				Message: "Hello, World! Updated",
			},
		})
	assert.NoErrorf(s.T(), err, "Error sending notification: %v", err)
}

func TestUnitTestSuite(t *testing.T) {
	suite.Run(t, new(UnitTestSuite))
}
