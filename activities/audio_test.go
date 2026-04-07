package activities

import (
	"os"
	"testing"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/transcode"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type AudioTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestActivityEnvironment
}

func (s *AudioTestSuite) SetupTest() {
	s.env = s.NewTestActivityEnvironment()
}

func (s *AudioTestSuite) TestTranscodeToAudioAac() {
	t := s.T()

	inputFile := paths.MustParse("./testdata/generated/aac_input.wav")
	outputDir := paths.MustParse("./testdata/generated/aac_output/")
	os.MkdirAll(outputDir.Local(), 0755)
	err := transcode.GenerateToneFile(440, 2, 48000, "01:00:00:00", inputFile)
	assert.NoError(t, err)

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.TranscodeToAudioAac)
	res, err := s.env.ExecuteActivity(aa.TranscodeToAudioAac, common.AudioInput{
		Path:            inputFile,
		Bitrate:         "128k",
		DestinationPath: outputDir,
	})
	assert.NoError(t, err)

	result := &common.AudioResult{}
	err = res.Get(result)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.OutputPath)
	assert.True(t, result.FileSize > 0)
}

func (s *AudioTestSuite) TestTranscodeToAudioWav() {
	t := s.T()

	inputFile := paths.MustParse("./testdata/generated/wav_input.wav")
	outputDir := paths.MustParse("./testdata/generated/wav_output/")
	os.MkdirAll(outputDir.Local(), 0755)
	err := transcode.GenerateToneFile(440, 2, 48000, "01:00:00:00", inputFile)
	assert.NoError(t, err)

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.TranscodeToAudioWav)
	res, err := s.env.ExecuteActivity(aa.TranscodeToAudioWav, common.WavAudioInput{
		Path:            inputFile,
		DestinationPath: outputDir,
	})
	assert.NoError(t, err)

	result := &common.AudioResult{}
	err = res.Get(result)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.OutputPath)
	assert.True(t, result.FileSize > 0)
}

func (s *AudioTestSuite) TestTranscodeToAudioWav_WithTimecode() {
	t := s.T()

	inputFile := paths.MustParse("./testdata/generated/wav_tc_input.wav")
	outputDir := paths.MustParse("./testdata/generated/wav_tc_output/")
	os.MkdirAll(outputDir.Local(), 0755)
	err := transcode.GenerateToneFile(440, 2, 48000, "01:00:00:00", inputFile)
	assert.NoError(t, err)

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.TranscodeToAudioWav)
	res, err := s.env.ExecuteActivity(aa.TranscodeToAudioWav, common.WavAudioInput{
		Path:            inputFile,
		DestinationPath: outputDir,
		Timecode:        "02:00:00:00",
	})
	assert.NoError(t, err)

	result := &common.AudioResult{}
	err = res.Get(result)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.OutputPath)
	assert.True(t, result.FileSize > 0)
}

func (s *AudioTestSuite) TestPrepareForTranscription_WithAudio() {
	t := s.T()

	testFile := paths.MustParse("./testdata/generated/transcription_input.mkv")
	outputDir := paths.MustParse("./testdata/generated/transcription_output/")
	os.MkdirAll(outputDir.Local(), 0755)
	testutils.GenerateSoftronTestFile(testFile, 2, 2)

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.PrepareForTranscription)
	s.env.RegisterActivity(aa.AnalyzeFile)
	res, err := s.env.ExecuteActivity(aa.PrepareForTranscription, common.AudioInput{
		Path:            testFile,
		DestinationPath: outputDir,
	})
	assert.NoError(t, err)

	result := &PrepareTranscriptionResult{}
	err = res.Get(result)
	assert.NoError(t, err)
	assert.True(t, result.HasAudio)
	assert.NotNil(t, result.OutputPath)
}

func (s *AudioTestSuite) TestPrepareForTranscription_NoAudio() {
	t := s.T()

	testFile := paths.MustParse("./testdata/generated/transcription_noaudio.mov")
	outputDir := paths.MustParse("./testdata/generated/transcription_noaudio_output/")
	os.MkdirAll(outputDir.Local(), 0755)
	testutils.GenerateVideoFile(testFile, testutils.VideoGeneratorParams{
		Duration:  2,
		FrameRate: 25,
		Width:     320,
		Height:    240,
		SAR:       "1:1",
		DAR:       "4:3",
		Profile:   "0",
	})

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.PrepareForTranscription)
	s.env.RegisterActivity(aa.AnalyzeFile)

	res, err := s.env.ExecuteActivity(aa.PrepareForTranscription, common.AudioInput{
		Path:            testFile,
		DestinationPath: outputDir,
	})
	assert.NoError(t, err)

	result := &PrepareTranscriptionResult{}
	err = res.Get(result)
	assert.NoError(t, err)
	assert.False(t, result.HasAudio)
	assert.Nil(t, result.OutputPath)
}

func (s *AudioTestSuite) TestDetectSilence_NotSilent() {
	t := s.T()

	testFile := paths.MustParse("./testdata/generated/silence_tone.wav")
	err := transcode.GenerateToneFile(440, 2, 48000, "01:00:00:00", testFile)
	assert.NoError(t, err)

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.DetectSilence)
	res, err := s.env.ExecuteActivity(aa.DetectSilence, common.DetectSilenceInput{
		Path: testFile,
	})
	assert.NoError(t, err)

	var isSilent bool
	err = res.Get(&isSilent)
	assert.NoError(t, err)
	assert.False(t, isSilent)
}

func (s *AudioTestSuite) TestExtractAudio() {
	t := s.T()

	testFile := paths.MustParse("./testdata/generated/extract_audio_input.mkv")
	outputDir := paths.MustParse("./testdata/generated/extract_audio_output/")
	os.MkdirAll(outputDir.Local(), 0755)
	testutils.GenerateSeparateAudioStreamsTestFile(testFile, 2, 2)

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.ExtractAudio)
	s.env.RegisterActivity(aa.AnalyzeFile)
	res, err := s.env.ExecuteActivity(aa.ExtractAudio, ExtractAudioInput{
		VideoPath:       testFile,
		OutputFolder:    outputDir,
		FileNamePattern: "audio_%d.wav",
	})
	assert.NoError(t, err)

	result := &ExtractAudioOutput{}
	err = res.Get(result)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.AudioFiles)
}

func (s *AudioTestSuite) TestExtractAudio_SpecificChannels() {
	t := s.T()

	testFile := paths.MustParse("./testdata/generated/extract_specific_input.mkv")
	outputDir := paths.MustParse("./testdata/generated/extract_specific_output/")
	os.MkdirAll(outputDir.Local(), 0755)
	testutils.GenerateSeparateAudioStreamsTestFile(testFile, 3, 2)

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.ExtractAudio)
	s.env.RegisterActivity(aa.AnalyzeFile)

	// Only extract the first audio channel (stream index depends on ordering;
	// with 3 audio + 1 video, audio streams are typically at indices 0, 1, 2)
	res, err := s.env.ExecuteActivity(aa.ExtractAudio, ExtractAudioInput{
		VideoPath:       testFile,
		OutputFolder:    outputDir,
		FileNamePattern: "audio_%d.wav",
		Channels:        []int{0},
	})
	assert.NoError(t, err)

	result := &ExtractAudioOutput{}
	err = res.Get(result)
	assert.NoError(t, err)
	assert.Len(t, result.AudioFiles, 1)
}

func (s *AudioTestSuite) TestExtractAudio_InvalidChannel() {
	t := s.T()

	testFile := paths.MustParse("./testdata/generated/extract_invalid_input.mkv")
	testutils.GenerateSeparateAudioStreamsTestFile(testFile, 2, 2)

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.ExtractAudio)
	s.env.RegisterActivity(aa.AnalyzeFile)
	_, err := s.env.ExecuteActivity(aa.ExtractAudio, ExtractAudioInput{
		VideoPath:       testFile,
		OutputFolder:    paths.MustParse("./testdata/generated/extract_invalid_output/"),
		FileNamePattern: "audio_%d.wav",
		Channels:        []int{99},
	})
	assert.Error(t, err)
}

func (s *AudioTestSuite) TestGenerateToneFile() {
	t := s.T()

	outputFile := paths.MustParse("./testdata/generated/tone_output.wav")
	os.MkdirAll(outputFile.Dir().Local(), 0755)

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.GenerateToneFile)
	res, err := s.env.ExecuteActivity(aa.GenerateToneFile, ToneInput{
		Frequency:       440,
		Duration:        2,
		SampleRate:      48000,
		TimeCode:        "01:00:00:00",
		DestinationFile: outputFile,
	})
	assert.NoError(t, err)

	result := &common.AudioResult{}
	err = res.Get(result)
	assert.NoError(t, err)
	assert.Equal(t, outputFile.Local(), result.OutputPath.Local())

	info, err := os.Stat(outputFile.Local())
	assert.NoError(t, err)
	assert.True(t, info.Size() > 0)
}

func TestAudioTestSuite(t *testing.T) {
	suite.Run(t, new(AudioTestSuite))
}
