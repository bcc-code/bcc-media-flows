package ingestworkflows

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/stretchr/testify/mock"

	//vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	//wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"encoding/json"
	"os"
	"testing"
	"time"

	//"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

func TestToSRT_SegmentLevel(t *testing.T) {
	segments := []Segment{
		{
			Start: 0.0, End: 2.5, Text: "Hello world!",
		},
		{
			Start: 3.0, End: 5.0, Text: "This is a test.",
		},
	}
	srt := ToSRT(segments, false)
	expected := `1
00:00:00,000 --> 00:00:02,500
Hello world!

2
00:00:03,000 --> 00:00:05,000
This is a test.

`
	if srt != expected {
		t.Errorf("Segment-level SRT output mismatch.\nGot:\n%s\nWant:\n%s", srt, expected)
	}
}

func TestToSRT_WordLevel(t *testing.T) {
	segments := []Segment{
		{
			Words: []Word{
				{Start: 0.0, End: 0.5, Text: "Hello"},
				{Start: 0.5, End: 1.0, Text: "world!"},
			},
		},
		{
			Words: []Word{
				{Start: 1.5, End: 2.0, Text: "Test"},
				{Start: 2.0, End: 2.5, Text: "again."},
			},
		},
	}
	srt := ToSRT(segments, true)
	expected := `1
00:00:00,000 --> 00:00:00,500
Hello

2
00:00:00,500 --> 00:00:01,000
world!

3
00:00:01,500 --> 00:00:02,000
Test

4
00:00:02,000 --> 00:00:02,500
again.

`
	if srt != expected {
		t.Errorf("Word-level SRT output mismatch.\nGot:\n%s\nWant:\n%s", srt, expected)
	}
}

// --- TEST SUITE FOR ImportSubtitles WORKFLOW ---

type ImportSubtitlesTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *ImportSubtitlesTestSuite) SetupTest() {
	os.Setenv("TEMPORAL_DEBUG", "true")
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *ImportSubtitlesTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *ImportSubtitlesTestSuite) Test_ImportSubtitlesWorkflow() {
	vxid := "VX-123"
	lang := "en"
	segments := []Segment{
		{Start: 0.0, End: 1.0, Text: "Hello"},
		{Start: 1.5, End: 2.5, Text: "World"},
	}
	input := ImportSubtitlesInput{
		VXID:      vxid,
		Language:  lang,
		Subtitles: Transcription{Segments: segments},
	}

	// Use a valid Path string for your environment
	//mockOutputPath := paths.MustParse("./testdata/output")

	s.env.OnActivity(activities.Vidispine.ImportFileAsShapeActivity, mock.Anything, mock.Anything).Return(&vsactivity.ImportFileResult{JobID: "job-srt"}, nil)
	s.env.OnActivity(activities.Vidispine.ImportFileAsShapeActivity, mock.Anything, mock.Anything).Return(&vsactivity.ImportFileResult{JobID: "job-json"}, nil)

	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Return("", nil)

	// Generate today's date for the path
	now := time.Now()
	datePath := now.Format("2006/01/02")
	testPath := "Production/aux/" + datePath + "/VX-123_subtitles"

	jsonString, err := json.MarshalIndent(input.Subtitles, "", "  ")
	if err != nil {
		s.T().Errorf("Error marshaling JSON: %v", err)
	}

	s.env.OnActivity(activities.Util.WriteFile, mock.Anything, activities.WriteFileInput{
		Path: paths.Path{Drive: paths.Drive{Value: "isilon"}, Path: testPath + ".json"},
		Data: []byte(jsonString),
	}).Return("", nil).Once()

	s.env.OnActivity(activities.Util.WriteFile, mock.Anything, activities.WriteFileInput{
		Path: paths.Path{Drive: paths.Drive{Value: "isilon"}, Path: testPath + ".srt"},
		Data: []byte("1\n00:00:00,000 --> 00:00:01,000\nHello\n\n2\n00:00:01,500 --> 00:00:02,500\nWorld\n\n"),
	}).Return("", nil).Once()

	s.env.OnActivity(activities.Vidispine.JobCompleteOrErr, mock.Anything, mock.Anything).Return(true, nil)

	//s.env.OnWorkflow("ImportFileAsSidecarActivity", mock.Anything, mock.Anything).Return(&vsactivity.ImportFileAsSidecarResult{}, nil)

	s.env.ExecuteWorkflow(ImportSubtitles, input)
	s.True(s.env.IsWorkflowCompleted())
	err = s.env.GetWorkflowError()
	s.NoError(err)
}

func TestImportSubtitlesTestSuite(t *testing.T) {
	suite.Run(t, new(ImportSubtitlesTestSuite))
}
