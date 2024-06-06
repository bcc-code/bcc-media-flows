package ingestworkflows

import (
	"encoding/json"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"testing"
)

type RawMaterialTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *RawMaterialTestSuite) SetupTest() {
	// Disable some timeout detection for easier debugging
	// This only works if set outside of the program!
	// os.Setenv("TEMPORAL_DEBUG", "true")

	s.env = s.NewTestWorkflowEnvironment()
}

func (s *RawMaterialTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

const rawMaterialFormJSON = `{"OrderForm":{"Value":"Rawmaterial"},"Targets":["test@example.com"],"Metadata":{"XMLName":{"Space":"","Local":"Metadata"},"JobProperty":{"JobID":6006,"UserName":"user.name","CompanyName":"","SourceIP":"1.2.3.4","UserEmail":"test@example.com","IngestStation":"filecatalyst-01/10.12.135.2","UploadBitRate":"39008 kbps","UploadTime":"00:32:42","FtpSiteID":"siteid","FileCount":4,"OrderForm":"Rawmaterial","SubmissionDate":"Wed Jun 05 15:39:38 CEST 2024","LastDateChanged":"Wed Jun 05 15:39:38 CEST 2024","Status":"pending","AssetType":"RAW","SenderEmail":"test@example.com","EpisodeTitle":"","EpisodeDescription":"","ProgramPost":"","ProgramID":"","Season":"","Episode":"","ReceivedFilename":"","PersonsAppearing":"","Tags":"","PromoType":"","Language":""},"FileList":{"Files":[{"FileName":"","IsFolder":false,"FileSize":29126656,"FilePath":"/files/6006"},{"FileName":"","IsFolder":false,"FileSize":25198336,"FilePath":"/files/6006"},{"FileName":"","IsFolder":false,"FileSize":9559295844,"FilePath":"/files/6006"},{"FileName":"","IsFolder":false,"FileSize":183379456,"FilePath":"/files/6006"}]},"JobHistoryLog":{"JobLogs":[{"LogID":0,"JobLogDate":"Wed Jun 05 15:39:38 CEST 2024","JobLogDescription":"status changed to 'submitted'","JobLogBy":""}]}},"Directory":{"Drive":"temp","Path":"workflows/314a5b8d-3f49-4797-bc56-8d32f11aefca/fc"}}`

func (s *RawMaterialTestSuite) Test_RawMaterialForm_InvalidFilename() {
	form := RawMaterialFormParams{}
	err := json.Unmarshal([]byte(rawMaterialFormJSON), &form)
	s.NoError(err)

	s.env.OnActivity(activities.Util.ListFiles, mock.Anything, mock.Anything).Return(
		paths.Files{
			paths.MustParse("./testdata/v i d e o.mxf"),
			paths.MustParse("./testdata/video.mp4"),
			paths.MustParse("./testdata/æøå.mp4"),
		}, nil)

	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).
		Once().
		Return("", nil)

	s.env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).
		Once().
		Return(nil, nil)

	s.env.OnActivity(activities.Util.SendEmail, mock.Anything, mock.Anything).
		Once().
		Return(nil, nil)

	s.env.ExecuteWorkflow(RawMaterialForm, form)
	s.True(s.env.IsWorkflowCompleted())

	err = s.env.GetWorkflowError()
	s.Equal(err.Error(), `workflow execution error (type: RawMaterialForm, workflowID: default-test-workflow-id, runID: default-test-run-id): invalid filename: {{test} v i d e o.mxf}`)
}

func Test_RawMaterialTestSuite(t *testing.T) {
	suite.Run(t, new(RawMaterialTestSuite))
}
