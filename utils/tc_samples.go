package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func TCToSamples(tc string, fps int, sampleRate int) (int, error) {
	frames, err := timecodeToFrames(tc, fps)
	if err != nil {
		return 0, err
	}
	return frames * sampleRate / fps, nil
}

func timecodeToFrames(timecode string, frameRate int) (int, error) {
	parts := strings.Split(timecode, ":")
	if len(parts) != 4 {
		return 0, fmt.Errorf("invalid timecode format")
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}

	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}

	seconds, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, err
	}

	frames, err := strconv.Atoi(parts[3])
	if err != nil {
		return 0, err
	}

	totalFrames := (hours*3600+minutes*60+seconds)*frameRate + frames
	return totalFrames, nil
}
