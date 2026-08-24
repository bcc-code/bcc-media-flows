package ingestworkflows

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ingest"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type UnitTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *UnitTestSuite) SetupTest() {
	// Disable some timeout detection for easier debugging
	s.T().Setenv("TEMPORAL_DEBUG", "true")

	s.env = s.NewTestWorkflowEnvironment()
}

func (s *UnitTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

// bulkVBMetadata is the order form a bulk VB delivery arrives with. It used to
// be read from testdata/BulkVB.xml; the fields are the ones Masters reads.
var bulkVBMetadata = &ingest.Metadata{
	JobProperty: ingest.JobProperty{
		JobID:            6434,
		OrderForm:        OrderFormVBMasterBulk.Value,
		AssetType:        "MAS",
		SenderEmail:      "test@example.com",
		ProgramID:        "TEMP - Testprosjekt",
		ProgramPost:      "01",
		ReceivedFilename: "MDTEST1",
		Language:         "MUL",
	},
}

func (s *UnitTestSuite) Test_VBBulk_MasterFlow() {
	testutils.GenerateVideoFile(paths.MustParse("./testdata/generated/VBBulk/VBBulk1.mxf"), testutils.VideoGeneratorParams{
		FrameRate: 25,
		Height:    1080,
		Width:     1920,
		Duration:  1,
		Profile:   "3",
	})

	testutils.GenerateVideoFile(paths.MustParse("./testdata/generated/VBBulk/VBBulk2.mxf"), testutils.VideoGeneratorParams{
		FrameRate: 25,
		Height:    1080,
		Width:     1920,
		Duration:  1,
		Profile:   "3",
	})

	bulkDir := paths.MustParse("./testdata/generated/VBBulk")
	params := MasterParams{
		OrderForm: OrderFormVBMasterBulk,
		Directory: &bulkDir,
		OutputDir: paths.MustParse("./testdata/generated/VBBulk_output"),
		Targets: []string{
			"test@example.com",
		},
		Metadata: bulkVBMetadata,
	}
	s.env.RegisterActivity(activities.Util.ListFiles)
	s.env.RegisterActivity(activities.Util.MoveFile)

	s.env.OnActivity(activities.Vidispine.CreatePlaceholderActivity, mock.Anything, vsactivity.CreatePlaceholderParams{Title: "VBBulk1.mxf"}).
		Once().
		Return(&vsactivity.CreatePlaceholderResult{AssetID: "VBBulk1"}, nil)

	s.env.OnActivity(activities.Vidispine.CreatePlaceholderActivity, mock.Anything, vsactivity.CreatePlaceholderParams{Title: "VBBulk2.mxf"}).
		Once().
		Return(&vsactivity.CreatePlaceholderResult{AssetID: "VBBulk2"}, nil)

	s.env.OnActivity(activities.Vidispine.ImportFileAsShapeActivity, mock.Anything, vsactivity.ImportFileAsShapeParams{
		AssetID:  "VBBulk1",
		FilePath: paths.MustParse("./testdata/generated/VBBulk_output/VBBulk1.mxf"),
		ShapeTag: "original",
		Growing:  false,
		Replace:  false,
	}).Once().Return(nil, nil)

	s.env.OnActivity(activities.Vidispine.ImportFileAsShapeActivity, mock.Anything, vsactivity.ImportFileAsShapeParams{
		AssetID:  "VBBulk2",
		FilePath: paths.MustParse("./testdata/generated/VBBulk_output/VBBulk2.mxf"),
		ShapeTag: "original",
		Growing:  false,
		Replace:  false,
	}).Once().Return(nil, nil)

	fileldsToSet := []vsapi.ItemMetadataFieldParams{
		{ItemID: "VBBulk1", Key: "portal_mf381829", Value: "test@example.com"},
		{ItemID: "VBBulk1", Key: "portal_mf846642", Value: "6434"},
		{ItemID: "VBBulk1", Key: "portal_mf189850", Value: "MUL"},
		{ItemID: "VBBulk1", Key: "portal_mf426791", Value: "Testprosjekt"},

		{ItemID: "VBBulk2", Key: "portal_mf381829", Value: "test@example.com"},
		{ItemID: "VBBulk2", Key: "portal_mf846642", Value: "6434"},
		{ItemID: "VBBulk2", Key: "portal_mf189850", Value: "MUL"},
		{ItemID: "VBBulk2", Key: "portal_mf426791", Value: "Testprosjekt"},
	}

	for _, field := range fileldsToSet {
		s.env.OnActivity(activities.Vidispine.SetVXMetadataFieldActivity, mock.Anything, field).Once().Return(nil, nil)
	}

	s.env.OnActivity(activities.Vidispine.JobCompleteOrErr, mock.Anything, mock.Anything).Times(2).Return(true, nil)

	s.env.OnWorkflow(miscworkflows.TranscribeVX, mock.Anything, miscworkflows.TranscribeVXInput{
		VXID:     "VBBulk1",
		Language: "no",
	}).Once().Return(nil, nil)

	s.env.OnWorkflow(miscworkflows.TranscribeVX, mock.Anything, miscworkflows.TranscribeVXInput{
		VXID:     "VBBulk2",
		Language: "no",
	}).Once().Return(nil, nil)

	s.env.OnWorkflow(miscworkflows.TranscodePreviewVX, mock.Anything, miscworkflows.TranscodePreviewVXInput{
		VXID: "VBBulk1",
	}).Once().Return(nil, nil)

	s.env.OnWorkflow(miscworkflows.TranscodePreviewVX, mock.Anything, miscworkflows.TranscodePreviewVXInput{
		VXID: "VBBulk2",
	}).Once().Return(nil, nil)

	s.env.OnActivity(activities.Util.SendEmail, mock.Anything, mock.Anything).Once().Return(nil, nil)

	s.env.ExecuteWorkflow(Masters, params)
	s.True(s.env.IsWorkflowCompleted())

	err := s.env.GetWorkflowError()
	s.NoError(err)
}

func TestUnitTestSuite(t *testing.T) {
	suite.Run(t, new(UnitTestSuite))
}
