package transcode

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/utils"
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

func parseProgressCallback(info *FFProbeResult, cb func(float64)) func(string) {
	return func(line string) {
		totalFrames, _ := strconv.ParseFloat(info.Streams[0].NbFrames, 64)
		duration := info.Streams[0].Tags.Duration
		layout := "15:04:05.999999999"
		t, err := time.Parse(layout, duration)
		var totalSeconds int
		if err == nil {
			totalSeconds = t.Hour()*3600 + t.Minute()*60 + t.Second()
		}

		parts := strings.Split(line, "=")

		if len(parts) != 2 {
			return
		}

		if totalFrames != 0 && parts[0] == "frame" {
			frame, _ := strconv.ParseFloat(parts[1], 64)
			if frame == 0 {
				cb(0)
			} else {
				cb(frame / totalFrames)
			}
		} else if totalSeconds != 0 && parts[0] == "out_time_us" {
			ms, _ := strconv.ParseFloat(parts[1], 64)
			if ms == 0 {
				cb(0)
			} else {
				cb(ms / float64(totalSeconds*1000*1000))
			}
		} else if parts[0] == "progress" {
			// Audio doesn't report progress in a conceivable way, so just return 1 on complete
			if parts[1] == "end" {
				cb(1)
			}
		}
	}
}

func ProRes(input ProResInput, progressCallback func(float64)) (*ProResResult, error) {
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

	fmt.Println(commandString)

	cmd := exec.Command("ffmpeg",
		strings.Split(
			commandString,
			" ",
		)...,
	)

	_, err = utils.ExecuteCmd(cmd, parseProgressCallback(info, progressCallback))
	if err != nil {
		return nil, fmt.Errorf("couldn't execute ffmpeg %s", err.Error())
	}

	return &ProResResult{
		OutputPath: outputPath,
	}, nil
}
