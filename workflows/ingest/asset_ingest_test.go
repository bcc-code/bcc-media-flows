package ingestworkflows

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"os"
	"testing"
)

type duplicatePathTestData struct {
	input    string
	expected string
}

func TestSanizizeDuplicatePaths(t *testing.T) {

	data := []duplicatePathTestData{
		{"1/2/3/4", "1/2/3/4"},
		{"1/2/3/4/4/3/2/1", "1/2/3/4/4/3/2/1"},
		{"/1/2/1/2//", "/1/2"},
		{"/files/5892/files/589", "/files/5892"},
	}

	for _, d := range data {
		result := sanitizeDuplicatdPath(d.input)
		assert.Equal(t, d.expected, result)
	}
}

type UnitTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *UnitTestSuite) SetupTest() {
	// Disable some timeout detection for easier debugging
	os.Setenv("TEMPORAL_DEBUG", "true")

	s.env = s.NewTestWorkflowEnvironment()
}

func (s *UnitTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *UnitTestSuite) Test_OtherMasters() {
	s.T().Skip("Not fully implemented")
	//s.env.OnActivity(SimpleActivity, mock.Anything, mock.Anything).Return(
	//"", errors.New("SimpleActivityFailure"))
	s.env.RegisterActivity(activities.Util.ReadFile)
	s.env.ExecuteWorkflow(Asset, AssetParams{XMLPath: "./testdata/OtherMasters.xml"})
	s.True(s.env.IsWorkflowCompleted())

	err := s.env.GetWorkflowError()
	s.NoError(err)
}

func TestUnitTestSuite(t *testing.T) {
	suite.Run(t, new(UnitTestSuite))
}
