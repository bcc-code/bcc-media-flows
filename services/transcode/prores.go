package transcode

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ProResInput struct {
	FilePath   string
	OutputDir  string
	Resolution string
	FrameRate  int
}

type ProResResult struct {
	OutputPath string
}

type Progress struct {
	Percent        float64 `json:"percent"`
	CurrentSeconds int     `json:"currentSeconds"`
	TotalSeconds   int     `json:"totalSeconds"`
	CurrentFrame   int     `json:"currentFrame"`
	TotalFrames    int     `json:"totalFrames"`
}

func parseProgressCallback(info *FFProbeResult, cb func(Progress)) func(string) {
	return func(line string) {
		totalFrames, _ := strconv.ParseFloat(info.Streams[0].NbFrames, 64)
		var totalSeconds int
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

		parts := strings.Split(line, "=")

		if len(parts) != 2 {
			return
		}

		var progress Progress

		if totalFrames != 0 && parts[0] == "frame" {
			frame, _ := strconv.ParseFloat(parts[1], 64)
			progress.TotalFrames = int(totalFrames)
			progress.CurrentFrame = int(frame)
			if frame == 0 {
				cb(progress)
			} else {
				progress.Percent = frame / totalFrames * 100
				cb(progress)
			}
		} else if totalSeconds != 0 && parts[0] == "out_time_us" {
			ms, _ := strconv.ParseFloat(parts[1], 64)
			progress.TotalSeconds = totalSeconds
			progress.CurrentSeconds = int(ms / 1000 / 1000)
			if ms == 0 {
				cb(progress)
			} else {
				progress.Percent = ms / float64(totalSeconds*1000*1000) * 100
				cb(progress)
			}
		} else if parts[0] == "progress" {
			// Audio doesn't report progress in a conceivable way, so just return 1 on complete
			progress.Percent = 100
			if parts[1] == "end" {
				cb(progress)
			}
		}
	}
}

func ProRes(input ProResInput, progressCallback func(Progress)) (*ProResResult, error) {
	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mov"
	outputPath := filepath.Join(input.OutputDir, filename)

	info, err := ProbeFile(input.FilePath)
	if err != nil {
		return nil, err
	}

	commandParts := []string{
		fmt.Sprintf("-i %s", input.FilePath),
		"-c:v prores_ks",
		"-progress pipe:1",
		"-profile:v 3",
		"-vendor ap10",
		"-vf setfield=tff",
		"-color_primaries bt709",
		"-color_trc bt709",
		"-colorspace bt709",
		"-y",
	}

	if input.Resolution != "" {
		commandParts = append(
			commandParts,
			fmt.Sprintf("-s %s", input.Resolution),
		)
	}

	if input.FrameRate != 0 {
		commandParts = append(
			commandParts,
			fmt.Sprintf("-r %d", input.FrameRate),
		)
	}

	commandParts = append(
		commandParts,
		"-bits_per_mb 8000",
		outputPath,
	)

	commandString := strings.Join(commandParts, " ")

	cmd := exec.Command("ffmpeg",
		strings.Split(
			commandString,
			" ",
		)...,
	)
	stdout, _ := cmd.StdoutPipe()

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	cb := parseProgressCallback(info, progressCallback)

	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
		cb(line)
	}

	err = cmd.Wait()
	if err != nil {
		return nil, err
	}

	return &ProResResult{
		OutputPath: outputPath,
	}, nil
}
