package ingestworkflows

import (
	"encoding/xml"
	"errors"
	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ingest"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"os"
	"testing"
	"time"
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
	s.env.RegisterActivity(activities.Util.ReadFile)
	s.env.ExecuteWorkflow(Asset, AssetParams{XMLPath: "./testdata/OtherMasters.xml"})
	s.True(s.env.IsWorkflowCompleted())

	err := s.env.GetWorkflowError()
	s.NoError(err)
}

func (s *UnitTestSuite) Test_VBBulk_AssetFlow() {

	// We need this because the file is moved in the flow
	err := testutils.CopyFile("./testdata/BulkVB.xml", "./testdata/generated/BulkVB.xml")
	s.NoError(err)

	s.env.RegisterActivity(activities.Util.ReadFile)
	s.env.RegisterActivity(activities.Util.MoveFile)
	s.env.RegisterActivity(activities.Util.DeletePath)

	s.env.OnActivity(activities.Util.SendEmail, mock.Anything, mock.Anything).Return(nil, nil)
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Return("", nil)
	s.env.OnActivity(activities.Util.RcloneCopyDir, mock.Anything, activities.RcloneCopyDirInput{
		Source:      "isilon:filecatalyst/workflow/files/6434",
		Destination: "isilon:temp/workflows/fc",
		Priority:    rclone.Priority{Value: "normal"},
	}).Return(1234, nil)
	s.env.OnActivity(activities.Util.RcloneWaitForJob, mock.Anything, activities.RcloneWaitForJobInput{
		JobID: 1234,
	}).Return(true, nil)

	xmlText, err := os.ReadFile("./testdata/generated/BulkVB.xml")
	var xmlDataDirty ingest.Metadata
	err = xml.Unmarshal(xmlText, &xmlDataDirty)
	s.NoError(err)
	xmlData := sanitizeOrderForm(&xmlDataDirty)

	// See continuation in Test_VBBulk_MasterFlow
	s.env.OnWorkflow(Masters, mock.Anything, MasterParams{
		OrderForm: OrderFormVBMasterBulk,
		Directory: &paths.Path{Path: "workflows/fc", Drive: paths.TempDrive},
		OutputDir: paths.Path{Path: "Production/masters/" + time.Now().Format("2006/01/02"), Drive: paths.IsilonDrive},
		Targets: []string{
			"test@example.com",
		},
		Metadata: xmlData,
	}).Once().Run(func(args mock.Arguments) {}).Return(nil, nil)

	s.env.ExecuteWorkflow(Asset, AssetParams{XMLPath: "./testdata/generated/BulkVB.xml"})
	s.True(s.env.IsWorkflowCompleted())

	err = s.env.GetWorkflowError()
	s.NoError(err)
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

	err := testutils.CopyFile("./testdata/BulkVB.xml", "./testdata/generated/BulkVB.xml")
	s.NoError(err)

	xmlText, err := os.ReadFile("./testdata/generated/BulkVB.xml")
	var xmlDataDirty ingest.Metadata
	err = xml.Unmarshal(xmlText, &xmlDataDirty)
	s.NoError(err)
	xmlData := sanitizeOrderForm(&xmlDataDirty)

	bulkDir := paths.MustParse("./testdata/generated/VBBulk")
	params := MasterParams{
		OrderForm: OrderFormVBMasterBulk,
		Directory: &bulkDir,
		OutputDir: paths.MustParse("./testdata/generated/VBBulk_output"),
		Targets: []string{
			"test@example.com",
		},
		Metadata: xmlData,
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

	err = s.env.GetWorkflowError()
	s.NoError(err)
}

func TestUnitTestSuite(t *testing.T) {
	suite.Run(t, new(UnitTestSuite))
}

// mockAssetUploadDeps mocks everything the Asset workflow needs for an Upload
// order form except the MoveUploadedFiles child, which each test sets up itself.
// MoveFile is mocked rather than registered so the fixture is read in place and
// nothing on disk is touched.
func (s *UnitTestSuite) mockAssetUploadDeps() {
	s.env.RegisterActivity(activities.Util.ReadFile)

	s.env.OnActivity(activities.Util.MoveFile, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	s.env.OnActivity(activities.Util.DeletePath, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	s.env.OnActivity(activities.Util.SendEmail, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Return("", nil).Maybe()
	s.env.OnActivity(activities.Util.RcloneCopyDir, mock.Anything, mock.Anything).Return(1234, nil).Maybe()
	s.env.OnActivity(activities.Util.RcloneWaitForJob, mock.Anything, mock.Anything).Return(true, nil).Maybe()
}

// A failing MoveUploadedFiles child must fail the workflow. Each switch case binds
// its own outputDir, so the error has to be checked inside the case — a check placed
// after the switch reads the outer err, which those cases never assign.
func (s *UnitTestSuite) Test_Upload_ChildWorkflowFailureIsReported() {
	s.mockAssetUploadDeps()

	s.env.OnWorkflow(MoveUploadedFiles, mock.Anything, mock.Anything).Once().
		Return(errors.New("could not move uploaded files"))

	s.env.ExecuteWorkflow(Asset, AssetParams{XMLPath: "./testdata/Upload.xml"})
	s.True(s.env.IsWorkflowCompleted())

	err := s.env.GetWorkflowError()
	s.Error(err, "a failed MoveUploadedFiles child must not be reported as success")
	s.Contains(err.Error(), "could not move uploaded files")
}

// The same order form must still succeed when the child succeeds.
func (s *UnitTestSuite) Test_Upload_Success() {
	s.mockAssetUploadDeps()

	s.env.OnWorkflow(MoveUploadedFiles, mock.Anything, mock.Anything).Once().Return(nil)

	s.env.ExecuteWorkflow(Asset, AssetParams{XMLPath: "./testdata/Upload.xml"})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

// Guards the switch in Asset. Every OrderForms member must be handled there, or
// it falls through to the default branch and is rejected at runtime. Adding a
// member without adding a case will fail this test instead of shipping an order
// form that quietly does nothing but copy files and send emails.
func TestEveryOrderFormIsHandledByAsset(t *testing.T) {
	handled := []OrderForm{
		OrderFormDistribution, // returns early, before the switch
		OrderFormRawMaterial,
		OrderFormSeriesMaster,
		OrderFormOtherMaster,
		OrderFormVBMaster,
		OrderFormVBMasterBulk,
		OrderFormLEDMaterial,
		OrderFormPodcast,
		OrderFormMultitrackPB,
		OrderFormUpload,
		OrderFormMusic,
	}

	assert.ElementsMatch(t, OrderForms.Members(), handled,
		"OrderForms changed: add a case to the switch in Asset and update this list")
}
