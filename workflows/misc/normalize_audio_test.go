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

// AnalyzeEBUR128Activity zeroes out adjustments it considers not worth making, so
// a zero suggestion must leave the file untouched. The previous condition
// (`SuggestedAdjustment <= 0.01`) instead ran a full re-encode with an adjustment
// of 0.0 dB.
func (s *NormalizeAudioTestSuite) Test_NormalizeAudio_NegligibleAdjustment_IsNotApplied() {
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Maybe().Return(nil, nil)

	inputPath := paths.MustParse("/mnt/isilon/test.wav")

	s.env.OnActivity(activities.Audio.AnalyzeEBUR128Activity, mock.Anything, activities.AnalyzeEBUR128Params{
		FilePath:       inputPath,
		TargetLoudness: -23,
	}).Return(&common.AnalyzeEBUR128Result{
		IntegratedLoudness:  -23.1,
		TruePeak:            -1.5,
		LoudnessRange:       5.0,
		SuggestedAdjustment: 0.0,
	}, nil)

	adjustCalls := 0
	s.env.OnActivity(activities.Audio.AdjustAudioLevelActivity, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { adjustCalls++ }).
		Return(&common.AudioResult{OutputPath: paths.MustParse("/mnt/temp/workflows/adjusted_test.wav")}, nil).
		Maybe()

	s.env.ExecuteWorkflow(NormalizeAudioLevelWorkflow, NormalizeAudioParams{
		FilePath:   "/mnt/isilon/test.wav",
		TargetLUFS: -23,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	s.Zero(adjustCalls, "a zero adjustment must not trigger a re-encode")

	var result NormalizeAudioResult
	s.env.GetWorkflowResult(&result)
	s.Equal(inputPath.Local(), result.FilePath, "the original file should be returned untouched")
	s.NotNil(result.InputAnalysis)
}

// A negative suggestion means the audio must be brought down, either because it is
// above target or because AnalyzeEBUR128Activity clamped it to stay under
// -0.9 dBTP.
func (s *NormalizeAudioTestSuite) Test_NormalizeAudio_LoudAudio_IsReduced() {
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Maybe().Return(nil, nil)

	inputPath := paths.MustParse("/mnt/isilon/test.wav")
	outputPath := paths.MustParse("/mnt/temp/workflows/adjusted_test.wav")

	s.env.OnActivity(activities.Audio.AnalyzeEBUR128Activity, mock.Anything, activities.AnalyzeEBUR128Params{
		FilePath:       inputPath,
		TargetLoudness: -23,
	}).Return(&common.AnalyzeEBUR128Result{
		IntegratedLoudness:  -18.0,
		TruePeak:            -1.0,
		LoudnessRange:       5.0,
		SuggestedAdjustment: -5.0, // negative: 5 dB too loud
	}, nil)

	var appliedAdjustment float64
	s.env.OnActivity(activities.Audio.AdjustAudioLevelActivity, mock.Anything, mock.MatchedBy(
		func(input activities.AdjustAudioLevelParams) bool {
			appliedAdjustment = input.Adjustment
			return true
		},
	)).Return(&common.AudioResult{OutputPath: outputPath, FileSize: 1024}, nil)

	s.env.ExecuteWorkflow(NormalizeAudioLevelWorkflow, NormalizeAudioParams{
		FilePath:   "/mnt/isilon/test.wav",
		TargetLUFS: -23,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	s.InDelta(-5.0, appliedAdjustment, 0.001)

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

// SuggestedAdjustment is TargetLoudness - IntegratedLoudness, so a positive value
// means the audio is too quiet and needs a boost. A skip condition written against
// the signed value rather than its magnitude excludes exactly these, leaving quiet
// audio below target.
func (s *NormalizeAudioTestSuite) Test_NormalizeAudio_QuietAudio_IsBoosted() {
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Maybe().Return(nil, nil)

	inputPath := paths.MustParse("/mnt/isilon/test.wav")
	outputPath := paths.MustParse("/mnt/temp/workflows/adjusted_test.wav")

	s.env.OnActivity(activities.Audio.AnalyzeEBUR128Activity, mock.Anything, activities.AnalyzeEBUR128Params{
		FilePath:       inputPath,
		TargetLoudness: -23,
	}).Return(&common.AnalyzeEBUR128Result{
		IntegratedLoudness:  -26.0,
		TruePeak:            -8.0,
		LoudnessRange:       5.0,
		SuggestedAdjustment: 3.0, // positive: 3 dB too quiet
	}, nil)

	var appliedAdjustment float64
	adjustCalls := 0
	s.env.OnActivity(activities.Audio.AdjustAudioLevelActivity, mock.Anything, mock.MatchedBy(
		func(input activities.AdjustAudioLevelParams) bool {
			appliedAdjustment = input.Adjustment
			adjustCalls++
			return true
		},
	)).Return(&common.AudioResult{OutputPath: outputPath, FileSize: 1024}, nil)

	s.env.ExecuteWorkflow(NormalizeAudioLevelWorkflow, NormalizeAudioParams{
		FilePath:   "/mnt/isilon/test.wav",
		TargetLUFS: -23,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	s.NotZero(adjustCalls, "quiet audio must be boosted, not skipped")
	// Passed through with its sign intact, so the boost goes up rather than down.
	s.InDelta(3.0, appliedAdjustment, 0.001)

	var result NormalizeAudioResult
	s.env.GetWorkflowResult(&result)
	s.Equal(outputPath.Local(), result.FilePath)
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
