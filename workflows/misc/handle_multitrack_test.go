package miscworkflows

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"testing"
	"time"
)

type HandleMultitrackTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *HandleMultitrackTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.SetTestTimeout(200 * time.Second)
}

func (s *HandleMultitrackTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *HandleMultitrackTestSuite) Test_makeLucidPath() {
	s.env.OnActivity(activities.Util.RcloneCopyFile, mock.Anything, activities.RcloneFileInput{
		Source:      paths.Path{Drive: paths.Drive{Value: "test"}, Path: "testing.wav"},
		Destination: paths.Path{Drive: paths.Drive{Value: "lucid"}, Path: "/01 Liveopptak fra Brunstad/01 RAW/" + time.Now().Format("2006-01-02")},
		Priority:    rclone.Priority{Value: "low"},
	}).Return(3, nil)

	s.env.OnActivity(activities.Util.RcloneWaitForJob, mock.Anything, activities.RcloneWaitForJobInput{
		JobID: 3,
	}).Return(true, nil)

	s.env.OnActivity(activities.Util.MoveFile, mock.Anything, activities.MoveFileInput{
		Source:      paths.Path{Drive: paths.Drive{Value: "test"}, Path: "testing.wav"},
		Destination: paths.Path{Drive: paths.Drive{Value: "isilon"}, Path: "/AudioArchive/" + time.Now().Format("2006/01/02")},
	}).Return(nil, nil)

	s.env.ExecuteWorkflow(HandleMultitrackFile, HandleMultitrackFileInput{
		Path: "./testdata/testing.wav",
	})
	s.True(s.env.IsWorkflowCompleted())

	err := s.env.GetWorkflowError()
	s.NoError(err)
}

func TestHandleMultitrackFile(t *testing.T) {
	suite.Run(t, new(HandleMultitrackTestSuite))
}
