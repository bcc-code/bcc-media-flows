package export

import (
	"encoding/json"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-platform/backend/asset"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"os"
	"testing"
)

type BMMExportTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *BMMExportTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *BMMExportTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *BMMExportTestSuite) Test_GenerateJSON() {
	jsonData, err := os.ReadFile("./testdata/bmm_chapter_export_input.json")
	s.NoError(err)
	s.NotEmpty(jsonData)

	params := VXExportChildWorkflowParams{}
	err = json.Unmarshal(jsonData, &params)
	s.NoError(err)

	audioResults := map[string][]common.AudioResult{}
	normalizedResults := map[string]activities.NormalizeAudioResult{}
	chapters := []asset.TimedMetadata{
		asset.TimedMetadata{
			ContentType:    "speech",
			Timestamp:      1907.7599999999948,
			Label:          "LABEL",
			Title:          "TITLE",
			Description:    "",
			SongNumber:     "",
			SongCollection: "",
			Highlight:      false,
			ImageFilename:  "",
			Persons:        []string{"PERSON"},
		},
	}

	s.env.ExecuteWorkflow(makeBMMJSON, params, audioResults, normalizedResults, chapters)
	s.True(s.env.IsWorkflowCompleted())
	err = s.env.GetWorkflowError()
	s.NoError(err)

	res := []byte{}
	s.env.GetWorkflowResult(&res)
	s.NotEmpty(res)

	d := BMMData{}
	err = json.Unmarshal(res, &d)
	s.NoError(err)

	s.Empty(d.Title)
	s.NotEmpty(d.PersonsAppearing)
	s.Equal("PERSON", d.PersonsAppearing[0])
}

func TestBMMExport(t *testing.T) {
	suite.Run(t, new(BMMExportTestSuite))
}
