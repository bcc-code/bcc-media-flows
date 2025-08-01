package transcode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"fmt"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	"github.com/bcc-code/mediabank-bridge/log"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const jData = `{"streams":[{"index":0,"codec_name":"mpeg2video","codec_long_name":"MPEG-2 video","profile":"4:2:2","codec_type":"video","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":1920,"height":1080,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":1,"sample_aspect_ratio":"1:1","display_aspect_ratio":"16:9","pix_fmt":"yuv422p","level":2,"color_space":"bt709","color_transfer":"bt709","color_primaries":"bt709","field_order":"tt","refs":1,"id":"","r_frame_rate":"25/1","avg_frame_rate":"25/1","time_base":"1/25","start_pts":0,"start_time":"0.000000","duration_ts":17041,"duration":"681.640000","bit_rate":"50000000","bits_per_raw_sample":"","nb_frames":"","channels":0,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":1,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":2,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":3,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":4,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":5,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":6,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":7,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":8,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":9,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":10,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":11,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":12,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":13,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":14,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":15,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}},{"index":16,"codec_name":"pcm_s24le","codec_long_name":"PCM signed 24-bit little-endian","profile":"","codec_type":"audio","codec_tag_string":"[0][0][0][0]","codec_tag":"0x0000","width":0,"height":0,"coded_width":0,"coded_height":0,"closed_captions":0,"film_grain":0,"has_b_frames":0,"sample_aspect_ratio":"","display_aspect_ratio":"","pix_fmt":"","level":0,"color_space":"","color_transfer":"","color_primaries":"","field_order":"","refs":0,"id":"","r_frame_rate":"0/0","avg_frame_rate":"0/0","time_base":"1/48000","start_pts":0,"start_time":"0.000000","duration_ts":32718720,"duration":"681.640000","bit_rate":"1152000","bits_per_raw_sample":"24","nb_frames":"","channels":1,"channel_layout":"","disposition":{"default":0,"dub":0,"original":0,"comment":0,"lyrics":0,"karaoke":0,"forced":0,"hearing_impaired":0,"visual_impaired":0,"clean_effects":0,"attached_pic":0,"timed_thumbnails":0,"captions":0,"descriptions":0,"metadata":0,"dependent":0,"still_image":0},"tags":{"creation_time":"0001-01-01T00:00:00Z","language":"","handler_name":"","vendor_id":"","encoder":"","timecode":"","DURATION":""}}],"format":{"filename":"/path/filename.mxf","nb_streams":17,"nb_programs":0,"format_name":"mxf","format_long_name":"MXF (Material eXchange Format)","start_time":"0.000000","duration":"681.640000","size":"5950135135","bit_rate":"69833168","probe_score":100,"tags":{"major_brand":"","minor_version":"","compatible_brands":"","creation_time":""}}}`

func Test_AudioAdioPreviewGenerator(t *testing.T) {
	log.ConfigureGlobalLogger(zerolog.DebugLevel)
	fileInfo := &ffmpeg.FFProbeResult{}
	err := json.Unmarshal([]byte(jData), fileInfo)
	assert.NoError(t, err)

	out, err := prepareAudioPreview(true, false, fileInfo, fileInfo.Format.Filename, "./temp/")
	assert.NoError(t, err)

	assert.ElementsMatch(t,
		[]string{
			"-i", "/path/filename.mxf",
			"-c:a", "aac", "-b:a", "64k", "-ar", "44100", "-ac", "2", "-profile:a", "aac_low",
			"-filter_complex", "[0:10]acopy[a10];[0:11]acopy[a11];[0:12]acopy[a12];[0:13]acopy[a13];[0:14]acopy[a14];[0:15]acopy[a15];[0:16]acopy[a16];[0:1][0:2]amerge=inputs=2[a1];[0:3][0:4]amerge=inputs=2[a3];[0:5][0:6]amerge=inputs=2[a5];[0:7][0:8]amerge=inputs=2[a7];[0:9]acopy[a9]",
			"-map", "[a7]", "temp/7.eng.aac",
			"-map", "[a9]", "temp/9.fra.aac",
			"-map", "[a10]", "temp/10.spa.aac",
			"-map", "[a14]", "temp/14.ron.aac",
			"-map", "[a12]", "temp/12.rus.aac",
			"-map", "[a15]", "temp/15.tur.aac",
			"-map", "[a16]", "temp/16.pol.aac",
			"-map", "[a5]", "temp/5.nld.aac",
			"-map", "[a3]", "temp/3.deu.aac",
			"-map", "[a1]", "temp/1.nor.aac",
			"-map", "[a11]", "temp/11.fin.aac",
			"-map", "[a13]", "temp/13.por.aac",
			"-y",
		}, out.FFMPEGParams)
	assert.Equal(t, map[string]string{
		"ron": "temp/14.ron.aac",
		"nor": "temp/1.nor.aac",
		"nld": "temp/5.nld.aac",
		"rus": "temp/12.rus.aac",
		"tur": "temp/15.tur.aac",
		"pol": "temp/16.pol.aac",
		"fin": "temp/11.fin.aac",
		"fra": "temp/9.fra.aac",
		"spa": "temp/10.spa.aac",
		"deu": "temp/3.deu.aac",
		"por": "temp/13.por.aac",
		"eng": "temp/7.eng.aac",
	}, out.LanguageMap)

}

func TestPreview_VUMeters_MultipleAudioTracks(t *testing.T) {
	t.Parallel()
	trackCounts := []int{1, 2, 4, 16}
	os.MkdirAll("testdata/generated", 0755)

	// Override the watermark path for testing
	oldWatermarkPath := previewWatermarkPath
	previewWatermarkPath = "testdata/test_overlay.png"
	defer func() { previewWatermarkPath = oldWatermarkPath }()

	for _, n := range trackCounts {
		t.Run("audio_tracks_"+string(rune(n)), func(t *testing.T) {
			inputFile := filepath.Join("testdata/generated", fmt.Sprintf("testsrc_%dtracks.mov", n))
			outputDir := "testdata/generated"
			p, err := paths.Parse(inputFile)
			require.NoError(t, err)
			testutils.GenerateSoftronTestFile(p, n, 2.0)

			previewInput := PreviewInput{
				FilePath:  inputFile,
				OutputDir: outputDir,
			}

			result, err := Preview(previewInput, nil)
			require.NoError(t, err, "Preview should succeed for %d tracks", n)
			require.NotNil(t, result)
			stat, err := os.Stat(result.LowResolutionPath)
			require.NoError(t, err)
			require.True(t, stat.Size() > 1000, "Preview output should not be empty for %d tracks", n)
		})
	}
}

func TestPreview_VUMeters_SeparateAudioStreams(t *testing.T) {
	t.Parallel()
	trackCounts := []int{1, 2, 4, 8}
	os.MkdirAll("testdata/generated", 0755)

	// Override the watermark path for testing
	oldWatermarkPath := previewWatermarkPath
	previewWatermarkPath = "testdata/test_overlay.png"
	defer func() { previewWatermarkPath = oldWatermarkPath }()

	for _, n := range trackCounts {
		t.Run(fmt.Sprintf("separate_streams_%d", n), func(t *testing.T) {
			inputFile := filepath.Join("testdata/generated", fmt.Sprintf("testsrc_separate_%dstreams.mov", n))
			outputDir := "testdata/generated"
			p, err := paths.Parse(inputFile)
			require.NoError(t, err)
			testutils.GenerateSeparateAudioStreamsTestFile(p, n, 2.0)

			previewInput := PreviewInput{
				FilePath:  inputFile,
				OutputDir: outputDir,
			}

			result, err := Preview(previewInput, nil)
			require.NoError(t, err, "Preview should succeed for %d separate streams", n)
			require.NotNil(t, result)
			stat, err := os.Stat(result.LowResolutionPath)
			require.NoError(t, err)
			require.True(t, stat.Size() > 1000, "Preview output should not be empty for %d separate streams", n)
		})
	}
}
