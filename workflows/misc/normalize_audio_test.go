package miscworkflows

import (
	"os"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type NormalizeAudioTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *NormalizeAudioTestSuite) SetupTest() {
	os.Setenv("TEMPORAL_DEBUG", "true")
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *NormalizeAudioTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *NormalizeAudioTestSuite) Test_NormalizeAudio_SmallAdjustment() {
	// When suggested adjustment is <= 0.01 dB, the workflow still applies it
	// (the condition in the workflow is: adjust when SuggestedAdjustment <= 0.01)
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Maybe().Return(nil, nil)

	outputPath := paths.MustParse("/mnt/temp/workflows/adjusted_test.wav")

	s.env.OnActivity(activities.Audio.AnalyzeEBUR128Activity, mock.Anything, activities.AnalyzeEBUR128Params{
		FilePath:       paths.MustParse("/mnt/isilon/test.wav"),
		TargetLoudness: -23,
	}).Return(&common.AnalyzeEBUR128Result{
		IntegratedLoudness:  -23.1,
		TruePeak:            -1.5,
		LoudnessRange:       5.0,
		SuggestedAdjustment: 0.0,
	}, nil)

	s.env.OnActivity(activities.Audio.AdjustAudioLevelActivity, mock.Anything, mock.Anything).
		Return(&common.AudioResult{
			OutputPath: outputPath,
			FileSize:   1024,
		}, nil)

	s.env.ExecuteWorkflow(NormalizeAudioLevelWorkflow, NormalizeAudioParams{
		FilePath:   "/mnt/isilon/test.wav",
		TargetLUFS: -23,
	})
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.NoError(err)

	var result NormalizeAudioResult
	s.env.GetWorkflowResult(&result)
	s.Equal(outputPath.Local(), result.FilePath)
	s.NotNil(result.InputAnalysis)
}

func (s *NormalizeAudioTestSuite) Test_NormalizeAudio_WithAdjustment() {
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Maybe().Return(nil, nil)

	inputPath := paths.MustParse("/mnt/isilon/test.wav")
	outputPath := paths.MustParse("/mnt/temp/workflows/adjusted_test.wav")

	s.env.OnActivity(activities.Audio.AnalyzeEBUR128Activity, mock.Anything, activities.AnalyzeEBUR128Params{
		FilePath:       inputPath,
		TargetLoudness: -23,
	}).Return(&common.AnalyzeEBUR128Result{
		IntegratedLoudness:  -30.0,
		TruePeak:            -10.0,
		LoudnessRange:       5.0,
		SuggestedAdjustment: -5.0, // Negative = needs adjustment (below threshold of 0.01)
	}, nil)

	s.env.OnActivity(activities.Audio.AdjustAudioLevelActivity, mock.Anything, mock.Anything).
		Return(&common.AudioResult{
			OutputPath: outputPath,
			FileSize:   1024,
		}, nil)

	s.env.ExecuteWorkflow(NormalizeAudioLevelWorkflow, NormalizeAudioParams{
		FilePath:   "/mnt/isilon/test.wav",
		TargetLUFS: -23,
	})
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.NoError(err)

	var result NormalizeAudioResult
	s.env.GetWorkflowResult(&result)
	s.Equal(outputPath.Local(), result.FilePath)
	s.NotNil(result.InputAnalysis)
}

func (s *NormalizeAudioTestSuite) Test_NormalizeAudio_WithOutputAnalysis() {
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Maybe().Return(nil, nil)

	inputPath := paths.MustParse("/mnt/isilon/test.wav")
	outputPath := paths.MustParse("/mnt/temp/workflows/adjusted_test.wav")

	// First call: analyze input
	s.env.OnActivity(activities.Audio.AnalyzeEBUR128Activity, mock.Anything, activities.AnalyzeEBUR128Params{
		FilePath:       inputPath,
		TargetLoudness: -23,
	}).Return(&common.AnalyzeEBUR128Result{
		IntegratedLoudness:  -30.0,
		TruePeak:            -10.0,
		LoudnessRange:       5.0,
		SuggestedAdjustment: -5.0,
	}, nil).Once()

	s.env.OnActivity(activities.Audio.AdjustAudioLevelActivity, mock.Anything, mock.Anything).
		Return(&common.AudioResult{
			OutputPath: outputPath,
			FileSize:   1024,
		}, nil)

	// Second call: analyze output
	s.env.OnActivity(activities.Audio.AnalyzeEBUR128Activity, mock.Anything, activities.AnalyzeEBUR128Params{
		FilePath:       outputPath,
		TargetLoudness: -23,
	}).Return(&common.AnalyzeEBUR128Result{
		IntegratedLoudness:  -23.2,
		TruePeak:            -5.0,
		LoudnessRange:       4.8,
		SuggestedAdjustment: 0.0,
	}, nil).Once()

	s.env.ExecuteWorkflow(NormalizeAudioLevelWorkflow, NormalizeAudioParams{
		FilePath:              "/mnt/isilon/test.wav",
		TargetLUFS:            -23,
		PerformOutputAnalysis: true,
	})
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.NoError(err)

	var result NormalizeAudioResult
	s.env.GetWorkflowResult(&result)
	s.NotNil(result.InputAnalysis)
	s.NotNil(result.OutputAnalysis)
	s.InDelta(-23.2, result.OutputAnalysis.IntegratedLoudness, 0.01)
}

func (s *NormalizeAudioTestSuite) Test_NormalizeAudio_SkippedWhenAboveThreshold() {
	// When SuggestedAdjustment > 0.01, no adjustment is applied
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Maybe().Return(nil, nil)

	s.env.OnActivity(activities.Audio.AnalyzeEBUR128Activity, mock.Anything, activities.AnalyzeEBUR128Params{
		FilePath:       paths.MustParse("/mnt/isilon/test.wav"),
		TargetLoudness: -23,
	}).Return(&common.AnalyzeEBUR128Result{
		IntegratedLoudness:  -23.5,
		TruePeak:            -1.5,
		LoudnessRange:       5.0,
		SuggestedAdjustment: 0.5, // Above 0.01 threshold, so adjustment is skipped
	}, nil)

	s.env.ExecuteWorkflow(NormalizeAudioLevelWorkflow, NormalizeAudioParams{
		FilePath:   "/mnt/isilon/test.wav",
		TargetLUFS: -23,
	})
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.NoError(err)

	var result NormalizeAudioResult
	s.env.GetWorkflowResult(&result)
	// File path should be the original since no adjustment was made
	expected := paths.MustParse("/mnt/isilon/test.wav")
	s.Equal(expected.Local(), result.FilePath)
}

func (s *NormalizeAudioTestSuite) Test_NormalizeAudio_AnalyzeError() {
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Maybe().Return(nil, nil)

	s.env.OnActivity(activities.Audio.AnalyzeEBUR128Activity, mock.Anything, mock.Anything).
		Return(nil, assert.AnError)

	s.env.ExecuteWorkflow(NormalizeAudioLevelWorkflow, NormalizeAudioParams{
		FilePath:   "/mnt/isilon/test.wav",
		TargetLUFS: -23,
	})
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)
}

func TestNormalizeAudioTestSuite(t *testing.T) {
	suite.Run(t, new(NormalizeAudioTestSuite))
}
