package activities

import (
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"os"
	"testing"
)

type TranscodeTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestActivityEnvironment
}

func (s *TranscodeTestSuite) SetupTest() {
	s.env = s.NewTestActivityEnvironment()
}

func (s *TranscodeTestSuite) TestAThing() {
	t := s.T()
	t.Skip("this test requires a special file that is rather large to commit")

	// Change this path to where your test file is
	testFilePath := "./testdata/5sec.mov"

	os.MkdirAll("./testdata/generated/", 0755)

	input := common.MergeInput{
		Title: "Softron_20sec_64ch_1audio-por",
		Items: []common.MergeInputItem{
			{
				Path:  paths.MustParse(testFilePath),
				Start: 0,
				End:   3,
				Streams: []vidispine.AudioStream{
					{
						StreamID:  2,
						ChannelID: 16,
					},
					{
						StreamID:  2,
						ChannelID: 17,
					},
				},
			},
		},
		OutputDir: paths.MustParse("./testdata/generated/"),
		WorkDir:   paths.MustParse("./testdata/generated/"),
		Duration:  3,
	}

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.TranscodeMergeAudio)
	res, err := s.env.ExecuteActivity(aa.TranscodeMergeAudio, input)

	assert.NoError(t, err)
	spew.Dump(res)
}

func TestTranscodeTestSuite(t *testing.T) {
	suite.Run(t, new(TranscodeTestSuite))
}
