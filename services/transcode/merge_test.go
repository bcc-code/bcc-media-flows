package transcode

import (
	"os"
	"os/exec"
	"testing"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	"github.com/stretchr/testify/assert"
)

func Test_mergeItemsToStereoStream_simple(t *testing.T) {
	//func mergeItemToStereoStream(index int, tag string, item common.MergeInputItem) (string, error) {

	str, err := mergeItemToStereoStream(0, "a0", common.MergeInputItem{
		Path: paths.MustParse("./testdata/test_tone_5s.wav"),
		Streams: []vidispine.AudioStream{
			vidispine.AudioStream{
				StreamID:  0,
				ChannelID: 0,
			},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, str, "[0:0]amerge=inputs=1[a0]")
}

func Test_mergeItemsToStereoStream_dualMono(t *testing.T) {
	//func mergeItemToStereoStream(index int, tag string, item common.MergeInputItem) (string, error) {

	file := testutils.GenerateDualMonoAudioFile(paths.MustParse("./testdata/5s_dual_mono.mov"), 1)

	str, err := mergeItemToStereoStream(0, "a0", common.MergeInputItem{
		Path: file,
		Streams: []vidispine.AudioStream{
			vidispine.AudioStream{
				StreamID:  0,
				ChannelID: 0,
			},
			vidispine.AudioStream{
				StreamID:  1,
				ChannelID: 0,
			},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, str, "[0:0][0:1]amerge=inputs=2[a0]")
}

func Test_mergeItemsToStereoStream_stereo(t *testing.T) {
	file := testutils.GenerateMultichannelAudioFile(paths.MustParse("./testdata/5s_stereo.wav"), 2, 1)

	str, err := mergeItemToStereoStream(0, "a0", common.MergeInputItem{
		Path: file,
		Streams: []vidispine.AudioStream{
			vidispine.AudioStream{
				StreamID:  0,
				ChannelID: 0,
			},
			vidispine.AudioStream{
				StreamID:  0,
				ChannelID: 1,
			},
			vidispine.AudioStream{
				StreamID:  1,
				ChannelID: 0,
			},
			vidispine.AudioStream{
				StreamID:  1,
				ChannelID: 1,
			},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, "[0:0]aselect[a0]", str)
}

// Test_mergeItemsToStereoStream_videoFirstStereoAudio reproduces the
// MEET_..._PGM_NOR.mov failure: a single stereo audio track sits behind a
// video (and optionally data) stream, so Vidispine's reported StreamID does
// not line up with ffmpeg's 0-based audio Index. The lookup must ignore
// non-audio streams and fall back to the 2-channel audio stream.
func Test_mergeItemsToStereoStream_videoFirstStereoAudio(t *testing.T) {
	outFile := paths.MustParse("./testdata/generated/1s_video_then_stereo.mov")
	_ = os.MkdirAll(outFile.Dir().Local(), 0755)

	ffArgs := []string{
		"-f", "lavfi",
		"-i", "testsrc=duration=1:size=320x240:rate=25",
		"-f", "lavfi",
		"-i", "sine=frequency=500:duration=1:sample_rate=48000",
		"-map", "0:v",
		"-map", "1:a",
		"-ac", "2",
		"-c:v", "mpeg2video",
		"-c:a", "pcm_s24le",
		"-y", outFile.Local(),
	}
	if output, err := exec.Command("ffmpeg", ffArgs...).CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg generation failed: %v\n%s", err, string(output))
	}

	// Simulate Vidispine reporting StreamID=1 for the audio component (it
	// does not match any audio stream's ffmpeg Index because the only audio
	// stream sits at Index 1 but the lookup is gated on audio-only streams,
	// whose 0-based indexing differs from the container-level numbering).
	str, err := mergeItemToStereoStream(0, "a0", common.MergeInputItem{
		Path: outFile,
		Streams: []vidispine.AudioStream{
			{StreamID: 1, ChannelID: 0},
			{StreamID: 1, ChannelID: 1},
		},
	})

	assert.NoError(t, err)
	// The stereo shortcut must reference the actual audio stream (Index 1),
	// not the video stream at Index 0.
	assert.Equal(t, "[0:1]aselect[a0]", str)
}

func Test_mergeItemsToStereoStream_64Chan(t *testing.T) {
	//func mergeItemToStereoStream(index int, tag string, item common.MergeInputItem) (string, error) {

	file := testutils.GenerateMultichannelAudioFile(paths.MustParse("./testdata/5s_64ch.wav"), 64, 1)

	str, err := mergeItemToStereoStream(0, "a0", common.MergeInputItem{
		Path: file,
		Streams: []vidispine.AudioStream{
			vidispine.AudioStream{
				StreamID:  0,
				ChannelID: 5,
			},
			vidispine.AudioStream{
				StreamID:  0,
				ChannelID: 6,
			},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, "[0:a]pan=stereo|c0=c5|c1=c6[a0]", str)
}

func Test_MergeSubtitles(t *testing.T) {
	output := paths.MustParse("./testdata/generated/")
	subPath := paths.MustParse("./testdata/sub1.srt")

	input := common.MergeInput{
		OutputDir: output,
		WorkDir:   output,
		Title:     t.Name(),
		Items: []common.MergeInputItem{
			common.MergeInputItem{
				Path:  subPath,
				Start: 0,
				End:   5,
			},
			common.MergeInputItem{
				Path:  subPath,
				Start: 10,
				End:   15,
			},
		},
	}

	res, err := MergeSubtitles(input, nil)
	assert.NoError(t, err)
	assert.Equal(t, paths.MustParse("./testdata/generated/Test_MergeSubtitles.srt"), res.Path)
	assert.FileExists(t, res.Path.Local())

	actual, _ := os.ReadFile(res.Path.Local())
	expected, _ := os.ReadFile("./testdata/subtitles_merge_result.srt")

	assert.Equal(t, expected, actual)
}

func Test_MergeSubtitlesByOffset(t *testing.T) {
	output := paths.MustParse("./testdata/generated/")
	subPath := paths.MustParse("./testdata/sub1.srt")

	input := common.MergeInput{
		OutputDir: output,
		WorkDir:   output,
		Title:     t.Name(),
		Items: []common.MergeInputItem{
			common.MergeInputItem{
				Path:        subPath,
				StartOffset: 0,
			},
			common.MergeInputItem{
				Path:        subPath,
				StartOffset: 100,
			},
		},
	}

	res, err := MergeSubtitlesByOffset(input, nil)
	assert.NoError(t, err)
	assert.Equal(t, paths.MustParse("./testdata/generated/Test_MergeSubtitlesByOffset.srt"), res.Path)
	assert.FileExists(t, res.Path.Local())

	actual, _ := os.ReadFile(res.Path.Local())
	expected, _ := os.ReadFile("./testdata/subtitles_merge_by_offset_result.srt")

	assert.Equal(t, expected, actual)
}

func Test_MergeSubtitles2(t *testing.T) {
	output := paths.MustParse("./testdata/generated/")
	subPath := paths.MustParse("./testdata/sub1.srt")

	input := common.MergeInput{
		OutputDir: output,
		WorkDir:   output,
		Title:     t.Name(),
		Items: []common.MergeInputItem{
			common.MergeInputItem{
				Path:  subPath,
				Start: 8,
				End:   15,
			},
		},
	}

	res, err := MergeSubtitles(input, nil)
	assert.NoError(t, err)
	assert.Equal(t, paths.MustParse("./testdata/generated/Test_MergeSubtitles2.srt"), res.Path)
	assert.FileExists(t, res.Path.Local())

	actual, _ := os.ReadFile(res.Path.Local())
	expected, _ := os.ReadFile("./testdata/Test_MergeSubtitles2.srt")

	assert.Equal(t, expected, actual)
}
