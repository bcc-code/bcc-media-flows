package transcode

import (
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	"github.com/stretchr/testify/assert"
	"testing"
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

func Test_mergeItemsToStereoStream_stero(t *testing.T) {
	//func mergeItemToStereoStream(index int, tag string, item common.MergeInputItem) (string, error) {

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
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, "[0:0][0:0]amerge=inputs=2[a0]", str)
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
