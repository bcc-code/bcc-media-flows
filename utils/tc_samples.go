package utils

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/orsinium-labs/enum"
)

type FrameRate enum.Member[string]

var (
	FrameRatePAL  = FrameRate{"PAL"}
	FrameRateNTSC = FrameRate{"NTSC"}
	FrameRates    = enum.New(FrameRateNTSC, FrameRatePAL)
)

func TCToSamples(tc string, fps int, sampleRate int) (int, error) {
	frames := 0
	var err error
	if strings.Contains(tc, "@") {

		splitTc := strings.Split(tc, "@")

		switch splitTc[1] {
		case FrameRatePAL.Value:
			fps = 25
		case FrameRateNTSC.Value:
			fps = 30
		default:
			return 0, fmt.Errorf("invalid frame rate")
		}

		frames, err = strconv.Atoi(splitTc[0])
	} else {
		frames, err = TimecodeToFrames(tc, fps)
	}

	if err != nil {
		return 0, err
	}
	return frames * sampleRate / fps, nil
}

func TimecodeToFrames(timecode string, frameRate int) (int, error) {
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
