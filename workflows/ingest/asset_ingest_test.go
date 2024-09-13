package ingestworkflows

import (
	"encoding/xml"
	"github.com/bcc-code/bcc-media-flows/activities"
	batonactivities "github.com/bcc-code/bcc-media-flows/activities/baton"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ingest"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	s.env.RegisterActivity(activities.Util.CreateFolder)
	s.env.RegisterActivity(activities.Util.RcloneCopyDir)
	s.env.RegisterActivity(activities.Util.RcloneWaitForJob)
	s.env.RegisterActivity(activities.Util.DeletePath)

	s.env.OnActivity(activities.Util.SendEmail, mock.Anything, mock.Anything).Return(nil, nil)

	xmlText, err := os.ReadFile("./testdata/generated/BulkVB.xml")
	var xmlDataDirty ingest.Metadata
	err = xml.Unmarshal(xmlText, &xmlDataDirty)
	s.NoError(err)
	xmlData := sanitizeOrderForm(&xmlDataDirty)

	// See continuation in Test_VBBulk_MasterFlow
	s.env.OnWorkflow(Masters, mock.Anything, MasterParams{
		OrderForm: OrderFormVBMasterBulk,
		Directory: paths.Path{Path: "workflows/fc", Drive: paths.TempDrive},
		OutputDir: paths.Path{Path: "Production/masters/2024/9/13", Drive: paths.IsilonDrive},
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
	})

	testutils.GenerateVideoFile(paths.MustParse("./testdata/generated/VBBulk/VBBulk2.mxf"), testutils.VideoGeneratorParams{
		FrameRate: 25,
		Height:    1080,
		Width:     1920,
		Duration:  1,
	})

	err := testutils.CopyFile("./testdata/BulkVB.xml", "./testdata/generated/BulkVB.xml")
	s.NoError(err)

	xmlText, err := os.ReadFile("./testdata/generated/BulkVB.xml")
	var xmlDataDirty ingest.Metadata
	err = xml.Unmarshal(xmlText, &xmlDataDirty)
	s.NoError(err)
	xmlData := sanitizeOrderForm(&xmlDataDirty)

	params := MasterParams{
		OrderForm: OrderFormVBMasterBulk,
		Directory: paths.MustParse("./testdata/generated/VBBulk"),
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

	s.env.OnActivity(batonactivities.QC, mock.Anything, mock.Anything).Twice().Return(nil, nil)

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
