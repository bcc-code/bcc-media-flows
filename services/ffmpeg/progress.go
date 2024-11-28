package ffmpeg

import (
	"strconv"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/samber/lo"
)

type ProgressCallback func(Progress)

type Progress struct {
	Params         string  `json:"command"`
	Percent        float64 `json:"percent"`
	CurrentSeconds int     `json:"currentSeconds"`
	TotalSeconds   float64 `json:"totalSeconds"`
	CurrentFrame   int     `json:"currentFrame"`
	TotalFrames    int     `json:"totalFrames"`
	Bitrate        string  `json:"bitrate"`
	Speed          string  `json:"speed"`
}

type StreamInfo struct {
	HasAudio     bool
	HasVideo     bool
	HasAlpha     bool
	VideoStreams []FFProbeStream
	AudioStreams []FFProbeStream
	SubSteams    []FFProbeStream
	OtherStreams []FFProbeStream
	Progressive  bool
	TotalFrames  int
	TotalSeconds float64
	FrameRate    int
	Height       int
	Width        int
}

func ProbeResultToInfo(info *FFProbeResult) StreamInfo {
	streamInfo := StreamInfo{
		HasAudio: lo.SomeBy(info.Streams, func(i FFProbeStream) bool {
			return i.CodecType == "audio"
		}),
		HasVideo: lo.SomeBy(info.Streams, func(i FFProbeStream) bool {
			return i.CodecType == "video"
		}),
		HasAlpha: lo.SomeBy(info.Streams, func(i FFProbeStream) bool {
			return utils.IsAlphaPixelFormat(i.PixFmt)
		}),
	}

	for _, stream := range info.Streams {
		switch stream.CodecType {
		case "audio":
			streamInfo.AudioStreams = append(streamInfo.AudioStreams, stream)
		case "video":
			streamInfo.VideoStreams = append(streamInfo.VideoStreams, stream)
		case "subtitle":
			streamInfo.SubSteams = append(streamInfo.SubSteams, stream)
		default:
			streamInfo.OtherStreams = append(streamInfo.OtherStreams, stream)
		}

	}

	stream, found := lo.Find(info.Streams, func(i FFProbeStream) bool {
		return i.CodecType == "video"
	})
	if !found {
		stream = info.Streams[0]
	}
	if streamInfo.HasVideo {
		streamInfo.Height = stream.Height
		streamInfo.Width = stream.Width
	}
	if info != nil {
		frames, _ := strconv.ParseInt(stream.NbFrames, 10, 64)
		streamInfo.TotalFrames = int(frames)
		duration := stream.Tags.Duration
		if duration != "" {
			layout := "15:04:05.999999999"
			t, err := time.Parse(layout, duration)
			if err == nil {
				streamInfo.TotalSeconds = float64(t.Hour()*3600 + t.Minute()*60 + t.Second())
			}
		}
		if streamInfo.TotalSeconds == 0 {
			floatSeconds, _ := strconv.ParseFloat(stream.Duration, 64)
			if floatSeconds != 0 {
				streamInfo.TotalSeconds = floatSeconds
			}
		}
		if stream.FieldOrder == "progressive" {
			streamInfo.Progressive = true
		}
	}

	if stream.RFrameRate != "" {
		parts := strings.Split(stream.RFrameRate, "/")
		if len(parts) == 2 {
			frames, _ := strconv.ParseFloat(parts[0], 64)
			seconds, _ := strconv.ParseFloat(parts[1], 64)
			if seconds != 0 {
				streamInfo.FrameRate = int(frames / seconds)
			}
		}
	}

	return streamInfo
}

func parseProgressCallback(command []string, info StreamInfo, cb func(Progress)) func(string) {
	var progress Progress

	progress.Params = strings.Join(command, " ")
	progress.TotalFrames = info.TotalFrames
	progress.TotalSeconds = info.TotalSeconds

	return func(line string) {

		parts := strings.Split(line, "=")

		if len(parts) != 2 {
			return
		}

		if parts[0] == "frame" {
			frame, _ := strconv.ParseInt(parts[1], 10, 64)
			progress.CurrentFrame = int(frame)
			if progress.TotalFrames != 0 && frame != 0 {
				progress.Percent = float64(frame) / float64(progress.TotalFrames) * 100
			}
		} else if parts[0] == "out_time_us" {
			ms, _ := strconv.ParseFloat(parts[1], 64)
			progress.CurrentSeconds = int(ms / 1000 / 1000)
			if progress.TotalSeconds != 0 && ms != 0 {
				progress.Percent = ms / (progress.TotalSeconds * 1000 * 1000) * 100
			}
		} else if parts[0] == "progress" {
			// Audio doesn't report progress in a conceivable way, so just return 1 on complete
			if parts[1] == "end" {
				progress.Percent = 100
			}
		} else if parts[0] == "bitrate" {
			progress.Bitrate = parts[1]
		} else if parts[0] == "speed" {
			progress.Speed = parts[1]
		}
		if parts[0] == "progress" && cb != nil {
			cb(progress)
		}
	}
}
