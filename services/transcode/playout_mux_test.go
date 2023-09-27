package transcode

import (
	"strings"
	"testing"

	"github.com/bcc-code/bccm-flows/common"
	"github.com/stretchr/testify/assert"
)

func Test_GenerateFFmpegParamsForPlayoutMux(t *testing.T) {
	const golden = `-progress pipe:1 -hide_banner -i root/BERG_TS01_ISRAEL_VOD.mxf -i root/BERG_TS01_ISRAEL_VOD-nor.wav -i root/BERG_TS01_ISRAEL_VOD-eng.wav -i root/BERG_TS01_ISRAEL_VOD-fin.wav -filter_complex [1:a]aresample=48000,channelsplit=channel_layout=stereo[nor_l][nor_r];[nor_l]asplit=10[nor_l_copy_0][nor_l_copy_1][nor_l_copy_2][nor_l_copy_3][nor_l_copy_4][nor_l_copy_5][nor_l_copy_6][nor_l_copy_7][nor_l_copy_8][nor_l_copy_9];[nor_r]asplit=3[nor_r_copy_0][nor_r_copy_1][nor_r_copy_2];[2:a]aresample=48000,channelsplit=channel_layout=stereo[eng_l][eng_r];[3:a]aresample=48000[fin_l] -map 0:v -map [nor_l_copy_0] -map [nor_r_copy_0] -map [nor_l_copy_1] -map [nor_r_copy_1] -map [nor_l_copy_2] -map [nor_r_copy_2] -map [eng_l] -map [eng_r] -map [nor_l_copy_3] -map [nor_l_copy_4] -map [fin_l] -map [nor_l_copy_5] -map [nor_l_copy_6] -map [nor_l_copy_7] -map [nor_l_copy_8] -map [nor_l_copy_9] -c:v copy -c:a pcm_s24le -y something/something.mxf`

	const root = "root/"
	const outputPath = "something/something.mxf"
	cmd, err := generateFFmpegParamsForPlayoutMux(common.PlayoutMuxInput{
		FileName:        "BERG_TS01_ISRAEL_VOD",
		StereoLanguages: []string{"nor", "eng", "fin"},
		DestinationPath: "transcoded/",
		VideoFilePath:   root + "BERG_TS01_ISRAEL_VOD.mxf",
		SubtitleFilePaths: map[string]string{
			"nor": root + "0.srt",
			"nld": root + "1.srt",
		},
		AudioFilePaths: map[string]string{
			"nor": root + "BERG_TS01_ISRAEL_VOD-nor.wav",
			"eng": root + "BERG_TS01_ISRAEL_VOD-eng.wav",
			"fin": root + "BERG_TS01_ISRAEL_VOD-fin.wav",
		},
	}, outputPath)

	assert.Nil(t, err)
	assert.Equal(t, strings.Join(cmd, " "), golden)
}

func Test_PlayoutMux(t *testing.T) {
	root := "/Users/andreasgangso/dev/div/520a9155-2c8f-4560-868b-53be9c6e9b96/"
	printer, stop := printProgress()
	defer close(stop)
	_, err := PlayoutMux(common.PlayoutMuxInput{
		FallbackLanguage: "nor",
		FileName:         "BERG_TS01_ISRAEL_VOD",
		StereoLanguages:  []string{"nor", "eng", "fin"},
		DestinationPath:  "/Users/andreasgangso/dev/div/520a9155-2c8f-4560-868b-53be9c6e9b96/transcoded/",
		VideoFilePath:    root + "BERG_TS01_ISRAEL_VOD.mxf",
		SubtitleFilePaths: map[string]string{
			"nor": root + "0.srt",
			"nld": root + "1.srt",
		},
		AudioFilePaths: map[string]string{
			"nor": root + "BERG_TS01_ISRAEL_VOD-nor.wav",
			"eng": root + "BERG_TS01_ISRAEL_VOD-eng.wav",
			"fin": root + "BERG_TS01_ISRAEL_VOD-fin.wav",
		},
	}, printer)

	assert.Nil(t, err)
}
