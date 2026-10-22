package vscommon

import (
	"errors"
	"strconv"
	"strings"
)

// TCToSeconds converts a Vidispine time code (<SAMPLES>@<TIMEBASE>) to seconds.
// The time base is PAL (25 fps) or a plain sample rate, e.g. "180000@50"
// for a 50 fps item or "48000@48000" for audio.
// http://10.12.128.15:8080/APIdoc/time.html#time-bases
func TCToSeconds(tc string) (float64, error) {
	splits := strings.Split(tc, "@")
	if len(splits) != 2 {
		return 0, errors.New("Invalid timecode: " + tc)
	}

	samples, err := strconv.ParseFloat(splits[0], 64)
	if err != nil {
		return 0, err
	}

	var rate float64
	switch splits[1] {
	case "PAL":
		rate = 25
	default:
		r, err := strconv.ParseInt(splits[1], 10, 64)
		if err != nil || r <= 0 {
			return 0, errors.New("Invalid timecode. Unsupported time base: " + tc)
		}
		rate = float64(r)
	}

	return samples / rate, nil
}
