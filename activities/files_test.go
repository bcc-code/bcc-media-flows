package activities

import (
	"os"
	"path"
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
	t := s.T()

	pathString := "./testdata/generated/a/b sdwef ,,_ /ss _.t xt.FFF"
	fileString := "asdk lkawd 823 ,, .xYz"
	err := os.MkdirAll(pathString, os.ModePerm)
	assert.NoError(t, err)

	// path.Join normalizes the path
	fullPath := "./" + path.Join(pathString, fileString)
	err = os.WriteFile(fullPath, []byte("test"), os.ModePerm)
	assert.NoError(t, err)

	p, err := paths.Parse(fullPath)
	assert.NoError(t, err)

	ua := UtilActivities{}

	s.env.RegisterActivity(ua.StandardizeFileName)
	res, err := s.env.ExecuteActivity(ua.StandardizeFileName, FileInput{Path: p})

	assert.NoError(t, err)
	assert.NotNil(t, res)

	res2 := &FileResult{}
	res.Get(res2)

	assert.NoError(t, err)
	assert.Equal(t, pathString+"/asdk_lkawd_823____.xYz", "./"+res2.Path.Local())
}

func TestUnitTestSuite(t *testing.T) {
	suite.Run(t, new(UnitTestSuite))
}
