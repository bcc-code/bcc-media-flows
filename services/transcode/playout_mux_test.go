package transcode

import (
	"strings"
	"testing"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/stretchr/testify/assert"
)

func Test_createStereoFilter(t *testing.T) {
	result := createStereoFilter("1:a", "nor_l", "nor_r")
	assert.Equal(t, "[1:a]aresample=48000,channelsplit=channel_layout=stereo[nor_l][nor_r]", result)
}

func Test_createMonoFilter(t *testing.T) {
	result := createMonoFilter("3:a", "fin_l")
	assert.Equal(t, "[3:a]aresample=48000,pan=1c|c0=c0[fin_l]", result)
}

func Test_createSplitFilter(t *testing.T) {
	filter, labels := createSplitFilter("nor_l", 3)
	assert.Equal(t, "[nor_l]asplit=3[nor_l_copy_0][nor_l_copy_1][nor_l_copy_2]", filter)
	assert.Equal(t, []string{"nor_l_copy_0", "nor_l_copy_1", "nor_l_copy_2"}, labels)
}

func Test_createSplitFilter_Single(t *testing.T) {
	filter, labels := createSplitFilter("eng_l", 1)
	assert.Equal(t, "[eng_l]asplit=1[eng_l_copy_0]", filter)
	assert.Equal(t, []string{"eng_l_copy_0"}, labels)
}

func Test_generateFFmpegParamsForPlayoutMux_MissingFallback(t *testing.T) {
	input := common.PlayoutMuxInput{
		VideoFilePath:    paths.MustParse("/mnt/isilon/test.mxf"),
		AudioFilePaths:   map[string]paths.Path{},
		FallbackLanguage: "nor",
	}

	_, err := generateFFmpegParamsForPlayoutMux(input, "/tmp/output.mxf")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fallback audio file not found")
}

func Test_generateFFmpegParamsForPlayoutMux_AllLanguages(t *testing.T) {
	audioPaths := map[string]paths.Path{}
	for _, lang := range playoutLanguages {
		audioPaths[lang] = paths.MustParse("/mnt/temp/audio_" + lang + ".wav")
	}

	input := common.PlayoutMuxInput{
		VideoFilePath:    paths.MustParse("/mnt/isilon/test.mxf"),
		AudioFilePaths:   audioPaths,
		FallbackLanguage: "nor",
	}

	params, err := generateFFmpegParamsForPlayoutMux(input, "/tmp/output.mxf")
	assert.NoError(t, err)

	joined := strings.Join(params, " ")

	// Should have video input + 12 audio inputs = 13 -i flags
	inputCount := strings.Count(joined, " -i ")
	assert.Equal(t, 13, inputCount)

	// Should map video first
	assert.Contains(t, joined, "-map 0:v")

	// Should have filter_complex
	assert.Contains(t, joined, "-filter_complex")

	// Should set output codec
	assert.Contains(t, joined, "-c:v copy")
	assert.Contains(t, joined, "-c:a pcm_s24le")

	// First 4 languages should produce stereo filters
	assert.Contains(t, joined, "channelsplit=channel_layout=stereo[nor_l][nor_r]")
	assert.Contains(t, joined, "channelsplit=channel_layout=stereo[deu_l][deu_r]")
	assert.Contains(t, joined, "channelsplit=channel_layout=stereo[nld_l][nld_r]")
	assert.Contains(t, joined, "channelsplit=channel_layout=stereo[eng_l][eng_r]")

	// Remaining languages should produce mono filters
	assert.Contains(t, joined, "pan=1c|c0=c0[fra_l]")
	assert.Contains(t, joined, "pan=1c|c0=c0[spa_l]")
}

func Test_generateFFmpegParamsForPlayoutMux_WithFallback(t *testing.T) {
	// Only provide nor and eng, rest should fall back to nor
	audioPaths := map[string]paths.Path{
		"nor": paths.MustParse("/mnt/temp/audio_nor.wav"),
		"eng": paths.MustParse("/mnt/temp/audio_eng.wav"),
	}

	input := common.PlayoutMuxInput{
		VideoFilePath:    paths.MustParse("/mnt/isilon/test.mxf"),
		AudioFilePaths:   audioPaths,
		FallbackLanguage: "nor",
	}

	params, err := generateFFmpegParamsForPlayoutMux(input, "/tmp/output.mxf")
	assert.NoError(t, err)

	joined := strings.Join(params, " ")

	// Should have video + 2 audio inputs = 3 -i flags
	inputCount := strings.Count(joined, " -i ")
	assert.Equal(t, 3, inputCount)

	// nor_l should be split because it is used as fallback for many languages
	assert.Contains(t, joined, "asplit=")

	// Output should still have the correct codec settings
	assert.Contains(t, joined, "-c:v copy")
	assert.Contains(t, joined, "-c:a pcm_s24le")
}
