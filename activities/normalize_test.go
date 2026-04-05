package activities

import (
	"os"
	"testing"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/transcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type NormalizeTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestActivityEnvironment
}

func (s *NormalizeTestSuite) SetupTest() {
	s.env = s.NewTestActivityEnvironment()
}

func (s *NormalizeTestSuite) TestAnalyzeEBUR128Activity() {
	t := s.T()

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.AnalyzeEBUR128Activity)

	// Use the existing test WAV file from the ffmpeg package
	testFile := paths.MustParse("./testdata/generated/ebur128_test.wav")
	err := transcode.GenerateToneFile(440, 3, 48000, "01:00:00:00", testFile)
	assert.NoError(t, err)

	res, err := s.env.ExecuteActivity(aa.AnalyzeEBUR128Activity, AnalyzeEBUR128Params{
		FilePath:       testFile,
		TargetLoudness: -23,
	})
	assert.NoError(t, err)

	result := &common.AnalyzeEBUR128Result{}
	err = res.Get(result)
	assert.NoError(t, err)

	// The tone file should have measurable loudness values
	assert.NotZero(t, result.IntegratedLoudness)
	assert.NotZero(t, result.TruePeak)
}

func (s *NormalizeTestSuite) TestAnalyzeEBUR128Activity_SuggestsAdjustment() {
	t := s.T()

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.AnalyzeEBUR128Activity)

	// Generate a quiet tone (low amplitude sine generates quiet audio)
	testFile := paths.MustParse("./testdata/generated/ebur128_quiet.wav")
	err := transcode.GenerateToneFile(440, 3, 48000, "01:00:00:00", testFile)
	assert.NoError(t, err)

	res, err := s.env.ExecuteActivity(aa.AnalyzeEBUR128Activity, AnalyzeEBUR128Params{
		FilePath:       testFile,
		TargetLoudness: -14, // target much louder than the tone
	})
	assert.NoError(t, err)

	result := &common.AnalyzeEBUR128Result{}
	err = res.Get(result)
	assert.NoError(t, err)

	// With a target of -14 and a typical generated tone around -20 to -25,
	// we should get a positive suggested adjustment
	assert.NotZero(t, result.SuggestedAdjustment)
}

func (s *NormalizeTestSuite) TestAdjustAudioLevelActivity() {
	t := s.T()

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.AdjustAudioLevelActivity)

	testFile := paths.MustParse("./testdata/generated/adjust_level_input.wav")
	outputDir := paths.MustParse("./testdata/generated/adjust_level_output/")
	os.MkdirAll(outputDir.Local(), 0755)
	err := transcode.GenerateToneFile(440, 3, 48000, "01:00:00:00", testFile)
	assert.NoError(t, err)

	res, err := s.env.ExecuteActivity(aa.AdjustAudioLevelActivity, AdjustAudioLevelParams{
		InFilePath:  testFile,
		OutFilePath: outputDir,
		Adjustment:  -3.0,
	})
	assert.NoError(t, err)

	result := &common.AudioResult{}
	err = res.Get(result)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.OutputPath)
	assert.True(t, result.FileSize > 0)
}

func (s *NormalizeTestSuite) TestNormalizeAudioActivity_WithAdjustment() {
	t := s.T()

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.NormalizeAudioActivity)
	s.env.RegisterActivity(aa.AnalyzeEBUR128Activity)
	s.env.RegisterActivity(aa.AdjustAudioLevelActivity)

	testFile := paths.MustParse("./testdata/generated/normalize_input.wav")
	outputDir := paths.MustParse("./testdata/generated/normalize_output/")
	os.MkdirAll(outputDir.Local(), 0755)
	err := transcode.GenerateToneFile(440, 3, 48000, "01:00:00:00", testFile)
	assert.NoError(t, err)

	res, err := s.env.ExecuteActivity(aa.NormalizeAudioActivity, NormalizeAudioParams{
		FilePath:              testFile,
		OutputPath:            outputDir,
		TargetLUFS:            -14, // Target much louder to trigger adjustment
		PerformOutputAnalysis: true,
	})
	assert.NoError(t, err)

	result := &NormalizeAudioResult{}
	err = res.Get(result)
	assert.NoError(t, err)
	assert.False(t, result.IsSilent)
}

func TestNormalizeTestSuite(t *testing.T) {
	suite.Run(t, new(NormalizeTestSuite))
}
