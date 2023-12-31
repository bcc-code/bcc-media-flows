package ffmpeg

import (
	"encoding/json"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/cache"
	"github.com/bcc-code/bcc-media-flows/utils"
	"os/exec"
	"time"
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
