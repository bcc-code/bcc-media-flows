package ffmpeg

import (
	"strconv"
	"strings"
	"time"
)

type ProgressCallback func(Progress)

type Progress struct {
	Percent        float64 `json:"percent"`
	CurrentSeconds int     `json:"currentSeconds"`
	TotalSeconds   float64 `json:"totalSeconds"`
	CurrentFrame   int     `json:"currentFrame"`
	TotalFrames    int     `json:"totalFrames"`
	Bitrate        string  `json:"bitrate"`
	Speed          string  `json:"speed"`
}

type StreamInfo struct {
	TotalFrames  int
	TotalSeconds float64
}

func ProbeResultToInfo(info *FFProbeResult) StreamInfo {
	var totalFrames int64
	var totalSeconds float64
	if info != nil {
		totalFrames, _ = strconv.ParseInt(info.Streams[0].NbFrames, 10, 64)
		duration := info.Streams[0].Tags.Duration
		if duration != "" {
			layout := "15:04:05.999999999"
			t, err := time.Parse(layout, duration)
			if err == nil {
				totalSeconds = float64(t.Hour()*3600 + t.Minute()*60 + t.Second())
			}
		}
		if totalSeconds == 0 {
			floatSeconds, _ := strconv.ParseFloat(info.Streams[0].Duration, 64)
			if floatSeconds != 0 {
				totalSeconds = floatSeconds
			}
		}
	}
	return StreamInfo{
		TotalFrames:  int(totalFrames),
		TotalSeconds: totalSeconds,
	}
}

func parseProgressCallback(info StreamInfo, cb func(Progress)) func(string) {
	var progress Progress

	return func(line string) {
		totalFrames := info.TotalFrames
		totalSeconds := info.TotalSeconds

		parts := strings.Split(line, "=")

		if len(parts) != 2 {
			return
		}

		if parts[0] == "frame" {
			frame, _ := strconv.ParseInt(parts[1], 10, 64)
			progress.TotalFrames = totalFrames
			progress.CurrentFrame = int(frame)
			if totalFrames != 0 && frame != 0 {
				progress.Percent = float64(frame) / float64(totalFrames) * 100
			}
		} else if parts[0] == "out_time_us" {
			ms, _ := strconv.ParseFloat(parts[1], 64)
			progress.TotalSeconds = totalSeconds
			progress.CurrentSeconds = int(ms / 1000 / 1000)
			if totalSeconds != 0 && ms != 0 {
				progress.Percent = ms / (totalSeconds * 1000 * 1000) * 100
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
		if parts[0] == "progress" {
			cb(progress)
		}
	}
}
