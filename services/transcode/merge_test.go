package transcode

import (
	"os"
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
