package ffmpeg

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/cache"
	"github.com/bcc-code/bcc-media-flows/utils"
)

type FFProbeStream struct {
	Index              int    `json:"index"`
	CodecName          string `json:"codec_name"`
	CodecLongName      string `json:"codec_long_name"`
	Profile            string `json:"profile"`
	CodecType          string `json:"codec_type"`
	CodecTagString     string `json:"codec_tag_string"`
	CodecTag           string `json:"codec_tag"`
	Width              int    `json:"width"`
	Height             int    `json:"height"`
	CodedWidth         int    `json:"coded_width"`
	CodedHeight        int    `json:"coded_height"`
	ClosedCaptions     int    `json:"closed_captions"`
	FilmGrain          int    `json:"film_grain"`
	HasBFrames         int    `json:"has_b_frames"`
	SampleAspectRatio  string `json:"sample_aspect_ratio"`
	DisplayAspectRatio string `json:"display_aspect_ratio"`
	PixFmt             string `json:"pix_fmt"`
	Level              int    `json:"level"`
	ColorSpace         string `json:"color_space"`
	ColorTransfer      string `json:"color_transfer"`
	ColorPrimaries     string `json:"color_primaries"`
	FieldOrder         string `json:"field_order"`
	Refs               int    `json:"refs"`
	Id                 string `json:"id"`
	RFrameRate         string `json:"r_frame_rate"`
	AvgFrameRate       string `json:"avg_frame_rate"`
	TimeBase           string `json:"time_base"`
	StartPts           int    `json:"start_pts"`
	StartTime          string `json:"start_time"`
	DurationTs         int    `json:"duration_ts"`
	Duration           string `json:"duration"`
	BitRate            string `json:"bit_rate"`
	BitsPerRawSample   string `json:"bits_per_raw_sample"`
	NbFrames           string `json:"nb_frames"`
	Channels           int    `json:"channels"`
	ChannelLayout      string `json:"channel_layout"`
	Disposition        struct {
		Default         int `json:"default"`
		Dub             int `json:"dub"`
		Original        int `json:"original"`
		Comment         int `json:"comment"`
		Lyrics          int `json:"lyrics"`
		Karaoke         int `json:"karaoke"`
		Forced          int `json:"forced"`
		HearingImpaired int `json:"hearing_impaired"`
		VisualImpaired  int `json:"visual_impaired"`
		CleanEffects    int `json:"clean_effects"`
		AttachedPic     int `json:"attached_pic"`
		TimedThumbnails int `json:"timed_thumbnails"`
		Captions        int `json:"captions"`
		Descriptions    int `json:"descriptions"`
		Metadata        int `json:"metadata"`
		Dependent       int `json:"dependent"`
		StillImage      int `json:"still_image"`
	} `json:"disposition"`
	Tags struct {
		CreationTime time.Time `json:"creation_time"`
		Language     string    `json:"language"`
		HandlerName  string    `json:"handler_name"`
		VendorId     string    `json:"vendor_id"`
		Encoder      string    `json:"encoder"`
		Timecode     string    `json:"timecode"`
		Duration     string    `json:"DURATION"`
	} `json:"tags"`
}

type FFProbeResult struct {
	Streams []FFProbeStream `json:"streams"`
	Format  struct {
		Filename       string `json:"filename"`
		NbStreams      int    `json:"nb_streams"`
		NbPrograms     int    `json:"nb_programs"`
		FormatName     string `json:"format_name"`
		FormatLongName string `json:"format_long_name"`
		StartTime      string `json:"start_time"`
		Duration       string `json:"duration"`
		Size           string `json:"size"`
		BitRate        string `json:"bit_rate"`
		ProbeScore     int    `json:"probe_score"`
		Tags           struct {
			MajorBrand       string `json:"major_brand"`
			MinorVersion     string `json:"minor_version"`
			CompatibleBrands string `json:"compatible_brands"`
			CreationTime     string `json:"creation_time"`
		} `json:"tags"`
	} `json:"format"`
}

func doProbe(path string) (*FFProbeResult, error) {

	cmd := exec.Command(
		"ffprobe",
		"-hide_banner",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)

	result, err := utils.ExecuteCmd(cmd, nil)
	if err != nil {
		return nil, fmt.Errorf("couldn't execute ffprobe %s, %s", path, err.Error())
	}

	var info FFProbeResult
	err = json.Unmarshal([]byte(result), &info)

	return &info, err
}

// ProbeFile returns information about the specified video file. Requires ffprobe present.
func ProbeFile(filePath string) (*FFProbeResult, error) {
	return cache.GetOrSet("probe:"+filePath, func() (*FFProbeResult, error) {
		return doProbe(filePath)
	})
}

func GetStreamInfo(path string) (StreamInfo, error) {
	info, err := ProbeFile(path)
	if err != nil {
		return StreamInfo{}, err
	}
	return ProbeResultToInfo(info), nil
}

// GetTimeCode returns the time code of the specified video file.
//
// Example:
// ffprobe MDTEST8_240122_MU1.mxf
// [mxf @ 0x559097e06700] index entry 2744 + TemporalOffset 1 = 2745, which is out of bounds
// [mxf @ 0x559097e06700] Estimating duration from bitrate, this may be inaccurate
// Input #0, mxf, from '/mnt/isilon/Production/raw/2024/1/22/4d9864f0-35a0-45cd-aaa3-ed0476887365/MDTEST8_240122_MU1.mxf':
//
//	Metadata:
//	  operational_pattern_ul: 060e2b34.04010101.0d010201.01010900
//	  uid             : adab4424-2f25-4dc7-92ff-000c00000000
//	  generation_uid  : adab4424-2f25-4dc7-92ff-000c00000001
//	  company_name    : FFmpeg
//	  product_name    : OP1a Muxer
//	  product_version_num: 60.3.100.0.0
//	  product_version : 60.3.100
//	  application_platform: Lavf (win32)
//	  product_uid     : adab4424-2f25-4dc7-92ff-29bd000c0002
//	  toolkit_version_num: 60.3.100.0.0
//	  material_package_umid: 0x060A2B340101010501010D00139C1F1952947134ED9C1F190052947134ED9C00
//	  timecode        : 13:50:38:05 <---------------------------------------------------------------- This is what we want
//	Duration: 00:01:52.00, start: 0.000000, bitrate: 68430 kb/s
func GetTimeCode(path string) (string, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-show_entries", "format_tags=timecode",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)

	res, err := utils.ExecuteCmd(cmd, nil)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(res), nil
}

// GetTimeReferencce returns the time reference of the specified wav file.
//
// Example:
// ffprobe 01-240122_1517.wav
// Input #0, wav, from '01-240122_1517.wav':
//
//	Metadata:
//	  encoded_by      : REAPER
//	  date            : 2024-01-22
//	  creation_time   : 15-17-09
//	  time_reference  : 2641753158   <----------------- This is what we want
//	Duration: 00:01:29.09, bitrate: 2304 kb/s
//	Stream #0:0: Audio: pcm_s24le ([1][0][0][0] / 0x0001), 48000 Hz, 2 channels, s32 (24 bit), 2304 kb/s
func GetTimeReferencce(path string) (int, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v", "quiet",
		"-show_entries", "format_tags=time_reference",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)

	samples, err := utils.ExecuteCmd(cmd, nil)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(strings.TrimSpace(samples))
}
