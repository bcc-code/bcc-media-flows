package export

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-platform/backend/asset"
	pcommon "github.com/bcc-code/bcc-media-platform/backend/common"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type BMMExportTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env               *testsuite.TestWorkflowEnvironment
	params            VXExportChildWorkflowParams
	normalizedResults map[string]activities.NormalizeAudioResult
	audioResults      map[string][]common.AudioResult
}

func (s *BMMExportTestSuite) SetupSuite() {
	jsonData, err := os.ReadFile("./testdata/bmm_chapter_export_input.json")
	s.NoError(err)
	s.NotEmpty(jsonData)

	err = json.Unmarshal(jsonData, &s.params)
	s.NoError(err)

	s.normalizedResults = map[string]activities.NormalizeAudioResult{}
	s.audioResults = map[string][]common.AudioResult{}
}

func (s *BMMExportTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *BMMExportTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

type testData struct {
	Chapters                 []asset.TimedMetadata
	ExpectedTitle            string
	ExpectedPersonsAppearing []string
	ExpecedSongNumber        *string
	ExpectedSongCollection   *string
}

func (s *BMMExportTestSuite) doTestGenerateJson(t testData) {
	s.env.ExecuteWorkflow(makeBMMJSON, s.params, s.audioResults, s.normalizedResults, t.Chapters)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.NoError(err)

	res := []byte{}
	s.env.GetWorkflowResult(&res)
	s.NotEmpty(res)

	d := BMMData{}
	err = json.Unmarshal(res, &d)
	s.NoError(err)

	s.Equal(t.ExpectedTitle, d.Title)
	s.Equal(t.ExpecedSongNumber, d.SongNumber)
	s.Equal(t.ExpectedSongCollection, d.SongCollection)
	s.Equal(t.ExpectedPersonsAppearing, d.PersonsAppearing)
}

func (s *BMMExportTestSuite) Test_GenerateJSON_Speech() {
	t := testData{
		Chapters: []asset.TimedMetadata{
			asset.TimedMetadata{
				ContentType:    pcommon.ContentTypeSpeech.Value,
				Timestamp:      1907.7599999999948,
				Label:          "LABEL",
				Title:          "TITLE",
				Description:    "",
				SongNumber:     "",
				SongCollection: "",
				Highlight:      false,
				Persons:        []string{"PERSON"},
			},
		},
		ExpectedTitle:            "",
		ExpectedPersonsAppearing: []string{"PERSON"},
		ExpecedSongNumber:        nil,
		ExpectedSongCollection:   nil,
	}

	s.doTestGenerateJson(t)
}

func (s *BMMExportTestSuite) Test_GenerateJSON_HVSong() {
	t := testData{
		Chapters: []asset.TimedMetadata{
			asset.TimedMetadata{
				ContentType:    pcommon.ContentTypeSong.Value,
				Timestamp:      1907.7599999999948,
				Label:          "LABEL",
				Title:          "TITLE",
				Description:    "",
				SongNumber:     "404",
				SongCollection: "HV",
				Highlight:      false,
				Persons:        []string{"PERSON"},
			},
		},
		ExpectedTitle:            "",
		ExpectedPersonsAppearing: []string{"PERSON"},
		ExpecedSongNumber:        aws.String("404"),
		ExpectedSongCollection:   aws.String("HV"),
	}
	s.doTestGenerateJson(t)
}

func (s *BMMExportTestSuite) Test_GenerateJSON_UnknownSong() {
	t := testData{
		Chapters: []asset.TimedMetadata{
			asset.TimedMetadata{
				ContentType:    pcommon.ContentTypeSong.Value,
				Timestamp:      1907.7599999999948,
				Label:          "SOME RANDOM SONG",
				Title:          "SONG TITLE",
				Description:    "",
				SongNumber:     "",
				SongCollection: "",
				Highlight:      false,
				Persons:        []string{"VOKALIST"},
			},
		},
		ExpectedTitle:            "SONG TITLE",
		ExpectedPersonsAppearing: []string{"VOKALIST"},
		ExpecedSongNumber:        nil,
		ExpectedSongCollection:   nil,
	}
	s.doTestGenerateJson(t)
}

func (s *BMMExportTestSuite) Test_GenerateJSON_SingAlong() {
	t := testData{
		Chapters: []asset.TimedMetadata{
			asset.TimedMetadata{
				ContentType:    pcommon.ContentTypeSingAlong.Value,
				Timestamp:      1907.7599999999948,
				Label:          "LABEL",
				Title:          "TITLE",
				Description:    "",
				SongNumber:     "404",
				SongCollection: "HV",
				Highlight:      false,
				Persons:        []string{},
			},
		},
		ExpectedTitle:            "",
		ExpectedPersonsAppearing: nil,
		ExpecedSongNumber:        aws.String("404"),
		ExpectedSongCollection:   aws.String("HV"),
	}
	s.doTestGenerateJson(t)
}
func (s *BMMExportTestSuite) Test_MakeBMMJSON_SkipsBrokenTranscriptions() {
	s.params.MergeResult.JSONTranscript = map[string]paths.Path{
		"en":  paths.New(paths.TestDrive, "english.srt"),
		"kha": paths.New(paths.TestDrive, "khasi.srt"),     // should be skipped
		"mal": paths.New(paths.TestDrive, "malayalam.srt"), // should be skipped
		"de":  paths.New(paths.TestDrive, "german.srt"),
	}

	chapters := []asset.TimedMetadata{
		{
			ContentType: pcommon.ContentTypeSpeech.Value,
		},
	}

	s.env.ExecuteWorkflow(makeBMMJSON, s.params, s.audioResults, s.normalizedResults, chapters)
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.NoError(err)

	res := []byte{}
	s.env.GetWorkflowResult(&res)
	s.NotEmpty(res)

	output := BMMData{}
	err = json.Unmarshal(res, &output)
	s.NoError(err)

	_, hasKha := output.TranscriptionFiles["kha"]
	_, hasMal := output.TranscriptionFiles["mal"]
	_, hasEng := output.TranscriptionFiles["eng"]
	_, hasDeu := output.TranscriptionFiles["deu"]

	s.False(hasKha, "kha should be skipped")
	s.False(hasMal, "mal should be skipped")
	s.True(hasEng, "eng should be present")
	s.True(hasDeu, "deu should be present")
}

func TestBMMExport(t *testing.T) {
	suite.Run(t, new(BMMExportTestSuite))
}
