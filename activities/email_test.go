package activities

import (
	"github.com/bcc-code/bcc-media-flows/services/emails"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"os"
	"testing"
)

type EmailTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestActivityEnvironment
}

func (s *EmailTestSuite) SetupTest() {
	s.env = s.NewTestActivityEnvironment()
}

func (s *EmailTestSuite) TestSimpleEmail() {
	t := s.T()
	if os.Getenv("SENDGRID_API_KEY") == "" {
		t.Skip("SENDGRID_API_KEY not set")
	}

	ua := UtilActivities{}
	s.env.RegisterActivity(ua.SendEmail)
	res, err := s.env.ExecuteActivity(ua.SendEmail, emails.Message{
		To:      []string{"67fe8ba8-9ddd-4808-891e-21a3c33ca94b@emailhook.site"},
		Subject: "Test",
		HTML:    "<p>Email from unit test</p>",
	})

	// You can verify that teh email was received at
	// https://webhook.site/#!/view/67fe8ba8-9ddd-4808-891e-21a3c33ca94b

	assert.NoError(t, err)
	assert.NotNil(t, res)
}

func TestEmailSuite(t *testing.T) {
	suite.Run(t, new(EmailTestSuite))
}
