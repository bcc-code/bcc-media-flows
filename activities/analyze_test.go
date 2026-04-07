package activities

import (
	"os"
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/services/transcode"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type AnalyzeTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestActivityEnvironment
}

func (s *AnalyzeTestSuite) SetupTest() {
	s.env = s.NewTestActivityEnvironment()
}

func (s *AnalyzeTestSuite) TestGetMimeType_TextFile() {
	t := s.T()

	testFile := paths.MustParse("./testdata/generated/mime_test.txt")
	os.MkdirAll(testFile.Dir().Local(), 0755)
	err := os.WriteFile(testFile.Local(), []byte("hello world"), 0644)
	assert.NoError(t, err)

	ua := UtilActivities{}
	s.env.RegisterActivity(ua.GetMimeType)
	res, err := s.env.ExecuteActivity(ua.GetMimeType, AnalyzeFileParams{
		FilePath: testFile,
	})
	assert.NoError(t, err)

	var mimeType *string
	err = res.Get(&mimeType)
	assert.NoError(t, err)
	assert.Contains(t, *mimeType, "text/plain")
}

func (s *AnalyzeTestSuite) TestGetMimeType_AudioFile() {
	t := s.T()

	testFile := paths.MustParse("./testdata/generated/mime_test.wav")
	err := transcode.GenerateToneFile(440, 1, 48000, "01:00:00:00", testFile)
	assert.NoError(t, err)

	ua := UtilActivities{}
	s.env.RegisterActivity(ua.GetMimeType)
	res, err := s.env.ExecuteActivity(ua.GetMimeType, AnalyzeFileParams{
		FilePath: testFile,
	})
	assert.NoError(t, err)

	var mimeType *string
	err = res.Get(&mimeType)
	assert.NoError(t, err)
	assert.Contains(t, *mimeType, "audio/")
}

func (s *AnalyzeTestSuite) TestAnalyzeFile_VideoWithAudio() {
	t := s.T()

	testFile := paths.MustParse("./testdata/generated/analyze_video.mkv")
	testutils.GenerateSoftronTestFile(testFile, 2, 2)

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.AnalyzeFile)
	res, err := s.env.ExecuteActivity(aa.AnalyzeFile, AnalyzeFileParams{
		FilePath: testFile,
	})
	assert.NoError(t, err)

	result := &ffmpeg.StreamInfo{}
	err = res.Get(result)
	assert.NoError(t, err)
	assert.True(t, result.HasVideo)
	assert.True(t, result.HasAudio)
	assert.Equal(t, 1920, result.Width)
	assert.Equal(t, 1080, result.Height)
	assert.NotEmpty(t, result.VideoStreams)
	assert.NotEmpty(t, result.AudioStreams)
}

func (s *AnalyzeTestSuite) TestAnalyzeFile_AudioOnly() {
	t := s.T()

	testFile := paths.MustParse("./testdata/generated/analyze_audio.wav")
	testutils.GenerateMultichannelAudioFile(testFile, 2, 2)

	aa := AudioActivities{}
	s.env.RegisterActivity(aa.AnalyzeFile)
	res, err := s.env.ExecuteActivity(aa.AnalyzeFile, AnalyzeFileParams{
		FilePath: testFile,
	})
	assert.NoError(t, err)

	result := &ffmpeg.StreamInfo{}
	err = res.Get(result)
	assert.NoError(t, err)
	assert.True(t, result.HasAudio)
	assert.False(t, result.HasVideo)
	assert.NotEmpty(t, result.AudioStreams)
	assert.Empty(t, result.VideoStreams)
}

func (s *AnalyzeTestSuite) TestGetVideoOffset() {
	t := s.T()

	video1 := paths.MustParse("./testdata/generated/offset_video1.mxf")
	video2 := paths.MustParse("./testdata/generated/offset_video2.mxf")
	testutils.GenerateVideoFileWithTimecode(video1, "01:00:00:00", 2, 25)
	testutils.GenerateVideoFileWithTimecode(video2, "01:00:01:00", 2, 25)

	va := VideoActivities{}
	s.env.RegisterActivity(va.GetVideoOffset)
	res, err := s.env.ExecuteActivity(va.GetVideoOffset, GetVideoOffsetInput{
		VideoPath1:      video1,
		VideoPath2:      video2,
		VideoFPS:        25,
		AudioSampleRate: 48000,
	})
	assert.NoError(t, err)

	var offset int
	err = res.Get(&offset)
	assert.NoError(t, err)

	// 1 second at 48000Hz = 48000 samples
	assert.Equal(t, 48000, offset)
}

func (s *AnalyzeTestSuite) TestGetVideoOffset_SameTimecode() {
	t := s.T()

	video1 := paths.MustParse("./testdata/generated/offset_same1.mxf")
	video2 := paths.MustParse("./testdata/generated/offset_same2.mxf")
	testutils.GenerateVideoFileWithTimecode(video1, "01:00:00:00", 2, 25)
	testutils.GenerateVideoFileWithTimecode(video2, "01:00:00:00", 2, 25)

	va := VideoActivities{}
	s.env.RegisterActivity(va.GetVideoOffset)
	res, err := s.env.ExecuteActivity(va.GetVideoOffset, GetVideoOffsetInput{
		VideoPath1:      video1,
		VideoPath2:      video2,
		VideoFPS:        25,
		AudioSampleRate: 48000,
	})
	assert.NoError(t, err)

	var offset int
	err = res.Get(&offset)
	assert.NoError(t, err)
	assert.Equal(t, 0, offset)
}

func TestAnalyzeTestSuite(t *testing.T) {
	suite.Run(t, new(AnalyzeTestSuite))
}
