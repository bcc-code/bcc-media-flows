package transcode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
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

func Test_AudioPreview_UnknownChannelFormat(t *testing.T) {
	fileInfo := &ffmpeg.FFProbeResult{}
	err := json.Unmarshal([]byte(jData), fileInfo)
	assert.NoError(t, err)

	// 16 audio streams but neither MU1 nor MU2: audio preview can't be built.
	// It must surface the sentinel so callers can skip audio preview while still
	// producing the video preview.
	out, err := prepareAudioPreview(false, false, fileInfo, fileInfo.Format.Filename, "./temp/")
	assert.Nil(t, out)
	assert.ErrorIs(t, err, ErrUnknownAudioChannelFormat)
}

func TestBuildVUMeterFilters_TRCPrefix(t *testing.T) {
	head, _ := buildVUMeterFilters(2, "setparams=color_trc=bt709,", "scale=1280:720")
	assert.Contains(t, head, "[0:v]setparams=color_trc=bt709,scale=1280:720[vmain]")

	head2, _ := buildVUMeterFilters(2, "", "scale=1280:720")
	assert.Contains(t, head2, "[0:v]scale=1280:720[vmain]")
	assert.NotContains(t, head2, "setparams")

	head3, _ := buildVUMeterFilters(2, "", "scale=-2:540")
	assert.Contains(t, head3, "[0:v]scale=-2:540[vmain]")
}

func TestBuildGrowingPreviewFilter(t *testing.T) {
	// No audio (or probe failure) falls back to the legacy filter without VU meters
	assert.Equal(t,
		"sws_flags=bicubic;[0:v]split=1[VIDEO-main-.mp4];[VIDEO-main-.mp4]scale=-2:540,null[temp];[temp][1:v]overlay=0:0:eof_action=repeat[VIDEO-.mp4];[0:a]aformat=channel_layouts=stereo[AUDIO-.mp4-0]",
		buildGrowingPreviewFilter(0, ""),
	)

	single := buildGrowingPreviewFilter(1, "")
	assert.Contains(t, single, "[0:a:0]showvolume=w=200:h=20:p=0.50:t=1,format=rgba[vum0]")
	assert.Contains(t, single, "[0:a:0]aformat=channel_layouts=stereo[AUDIO-.mp4-0]")

	assert.Equal(t,
		"sws_flags=bicubic;"+
			"[0:v]setparams=color_trc=bt709,scale=-2:540[vmain];"+
			"[0:a:0]showvolume=w=200:h=20:p=0.50:t=1,format=rgba[vum0];"+
			"[vmain][vum0]overlay=x=10:y=10[tmp0];"+
			"[0:a:1]showvolume=w=200:h=20:p=0.50:t=1,format=rgba[vum1];"+
			"[tmp0][vum1]overlay=x=10:y=40[tmp1];"+
			"[tmp1][1:v]overlay=0:0:eof_action=repeat[VIDEO-.mp4];"+
			"[0:a:0][0:a:1]amerge=inputs=2,pan=stereo|c0<c0|c1<c1[AUDIO-.mp4-0]",
		buildGrowingPreviewFilter(2, "setparams=color_trc=bt709,"),
	)

	many := buildGrowingPreviewFilter(16, "")
	assert.Equal(t, 16, strings.Count(many, "showvolume"))
	assert.Contains(t, many, "overlay=x=10:y=460[tmp15]")
	assert.Contains(t, many, "[0:a:0][0:a:1]amerge=inputs=2,pan=stereo|c0<c0|c1<c1[AUDIO-.mp4-0]")
}

// growingPreviewFailFast runs GrowingPreview against a file that does not exist, which
// makes both children exit within about a second: tail fails immediately, its stdout
// closes, and ffmpeg then has an empty input to probe.
//
// This deliberately avoids generated media. Feeding ffmpeg an unprobeable stream does
// not work as a trigger — `tail -f` never closes the pipe, so ffmpeg blocks waiting for
// a header rather than failing, which is also why a missing watermark is no use: ffmpeg
// never gets past probing input 0 to discover input 1 is missing.
func growingPreviewFailFast(t *testing.T) error {
	t.Helper()

	tempDir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- GrowingPreview(ctx, GrowingPreviewInput{
			FilePath:        filepath.Join(tempDir, "does-not-exist.mxf"),
			TempDir:         tempDir,
			DestinationFile: filepath.Join(tempDir, "preview.mp4"),
			WatermarkPath:   "testdata/test_overlay.png",
		}, func(ctx context.Context, duration time.Duration) {})
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(30 * time.Second):
		t.Fatal("GrowingPreview did not notice ffmpeg exiting; it waited for cancellation instead")
		return nil
	}
}

// ffmpeg exiting on its own means no more preview is coming, so GrowingPreview must
// return promptly. Nothing used to watch for it: the loop selected only on a 60s timer
// and ctx.Done(), so the activity kept heartbeating and remuxing a stale playlist until
// its 8 hour StartToCloseTimeout. Note the context is never cancelled here.
func TestGrowingPreview_FfmpegExitingEarlyIsReported(t *testing.T) {
	err := growingPreviewFailFast(t)

	require.Error(t, err, "a dead ffmpeg must be reported, not waited out")
	assert.Contains(t, err.Error(), "ffmpeg exited before the ingest finished")
	// The stderr tail is what makes the failure diagnosable at all.
	assert.Contains(t, err.Error(), "ffmpeg stderr")
}

// countOpenFDs reports how many descriptors this process holds.
//
// Linux exposes this as /proc/self/fd, which is where the workers run. macOS has
// /dev/fd but os.ReadDir on it fails with EBADF, so this skips there rather than
// pretending to measure something.
func countOpenFDs(t *testing.T) int {
	t.Helper()

	for _, dir := range []string{"/proc/self/fd", "/dev/fd"} {
		entries, err := os.ReadDir(dir)
		if err == nil {
			return len(entries)
		}
	}

	t.Skip("no readable fd directory on this platform (expected on macOS; runs on Linux)")
	return 0
}

// Each invocation used to leak the tail pipe's read end and leave an unreaped tail:
// StdoutPipe registers the read end in tailCmd.parentIOPipes, which only Wait closes,
// and Wait was never called. That leak is also the only thing that could ever have made
// os.Pipe fail, which the old code answered with os.Exit(1).
func TestGrowingPreview_DoesNotLeakDescriptors(t *testing.T) {
	// Warm up first so one-off allocations are not counted as growth.
	_ = growingPreviewFailFast(t)
	before := countOpenFDs(t)

	const iterations = 5
	for i := 0; i < iterations; i++ {
		_ = growingPreviewFailFast(t)
	}

	after := countOpenFDs(t)
	assert.LessOrEqual(t, after, before+1,
		"descriptors grew from %d to %d over %d runs, so the tail pipe is still leaking",
		before, after, iterations)
}
