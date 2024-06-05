package wfutils

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"testing"
	"time"
)

type UnitTestEncoding struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *UnitTestEncoding) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterActivity(activities.Util.ReadFile)
}

func (s *UnitTestEncoding) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

//func NowTestWF(ctx workflow.Context) (time.Time, error) {
//return Now(ctx), nil
//}

type testStruct struct {
	Name  string `xml:"name" json:"name"`
	Thing int    `xml:"thing" json:"thing"`
}

func MarshalXMLTest(ctx workflow.Context) ([]byte, error) {
	return MarshalXml(ctx, testStruct{
		Name:  "test",
		Thing: 1,
	})
}

func MarshalJSONTest(ctx workflow.Context) ([]byte, error) {
	return MarshalJson(ctx, testStruct{
		Name:  "test",
		Thing: 1,
	})
}

func UnmarshalJSONTest(ctx workflow.Context) (*testStruct, error) {
	return UnmarshalJson[testStruct](ctx, []byte("{\"name\":\"test\",\"thing\":1}"))
}

func UnmarshalXMLFileTest(ctx workflow.Context) (*testStruct, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second})
	return UnmarshalXMLFile[testStruct](ctx, paths.MustParse("./testdata/test.xml"))
}

func (s *UnitTestEncoding) Test_MarshalXML() {
	s.env.ExecuteWorkflow(MarshalXMLTest)

	var t []byte
	err := s.env.GetWorkflowResult(&t)

	s.NotEmpty(t)
	s.NoError(err)
	s.Equal("<testStruct>\n\t<name>test</name>\n\t<thing>1</thing>\n</testStruct>", string(t))

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UnitTestEncoding) Test_UnmarshalXMLFile() {
	s.env.ExecuteWorkflow(UnmarshalXMLFileTest)

	var t testStruct
	err := s.env.GetWorkflowResult(&t)

	s.NotEmpty(t)
	s.NoError(err)
	s.Equal(testStruct{
		Name:  "test",
		Thing: 1,
	}, t)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UnitTestEncoding) Test_MarshalJSON() {
	s.env.ExecuteWorkflow(MarshalJSONTest)

	var t []byte
	err := s.env.GetWorkflowResult(&t)

	s.NotEmpty(t)
	s.NoError(err)
	s.Equal("{\"name\":\"test\",\"thing\":1}", string(t))

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UnitTestEncoding) Test_UnmarshalJSON() {
	s.env.ExecuteWorkflow(UnmarshalJSONTest)

	var t testStruct
	err := s.env.GetWorkflowResult(&t)

	s.NotEmpty(t)
	s.NoError(err)
	s.Equal(testStruct{
		Name:  "test",
		Thing: 1,
	}, t)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func TestEncodingSuite(t *testing.T) {
	suite.Run(t, new(UnitTestEncoding))
}
