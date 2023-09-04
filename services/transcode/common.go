package transcode

import (
	"strconv"
	"strings"
	"time"
)

func parseProgressCallback(info *FFProbeResult, cb func(Progress)) func(string) {
	var progress Progress

	return func(line string) {
		var totalFrames float64
		var totalSeconds int
		if info != nil {
			totalFrames, _ = strconv.ParseFloat(info.Streams[0].NbFrames, 64)
			duration := info.Streams[0].Tags.Duration
			if duration != "" {
				layout := "15:04:05.999999999"
				t, err := time.Parse(layout, duration)
				if err == nil {
					totalSeconds = t.Hour()*3600 + t.Minute()*60 + t.Second()
				}
			}
			if totalSeconds == 0 {
				floatSeconds, _ := strconv.ParseFloat(info.Streams[0].Duration, 64)
				if floatSeconds != 0 {
					totalSeconds = int(floatSeconds)
				}
			}
		}

		parts := strings.Split(line, "=")

		if len(parts) != 2 {
			return
		}

		if totalFrames != 0 && parts[0] == "frame" {
			frame, _ := strconv.ParseFloat(parts[1], 64)
			progress.TotalFrames = int(totalFrames)
			progress.CurrentFrame = int(frame)
			if frame != 0 {
				progress.Percent = frame / totalFrames * 100
			}
		} else if totalSeconds != 0 && parts[0] == "out_time_us" {
			ms, _ := strconv.ParseFloat(parts[1], 64)
			progress.TotalSeconds = totalSeconds
			progress.CurrentSeconds = int(ms / 1000 / 1000)
			if ms != 0 {
				progress.Percent = ms / float64(totalSeconds*1000*1000) * 100
			}
		} else if parts[0] == "progress" {
			// Audio doesn't report progress in a conceivable way, so just return 1 on complete
			if parts[1] == "end" {
				progress.Percent = 100
			}
		} else if parts[0] == "bitrate" {
			progress.Bitrate = parts[1]
		}
		if parts[0] == "progress" {
			cb(progress)
		}
	}
}
