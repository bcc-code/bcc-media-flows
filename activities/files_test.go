package activities

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type UnitTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestActivityEnvironment
}

func (s *UnitTestSuite) SetupTest() {
	s.env = s.NewTestActivityEnvironment()
}

func (s *UnitTestSuite) TestStandardizeFileName() {
	p, err := paths.Parse("/mnt/filecatalyst/a  b /c/d e g.txt")
	t := s.T()

	ua := UtilActivities{}
	assert.NoError(t, err)

	s.env.RegisterActivity(ua.StandardizeFileName)
	res, err := s.env.ExecuteActivity(ua.StandardizeFileName, FileInput{Path: p})

	assert.NoError(t, err)
	assert.NotNil(t, res)

	p2 := &paths.Path{}
	res.Get(p2)

	assert.NoError(t, err)
	assert.Equal(t, "/a/b/c/d_e_g.txt", p2.Path)
}

func TestUnitTestSuite(t *testing.T) {
	suite.Run(t, new(UnitTestSuite))
}
