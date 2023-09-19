package vscommon

import (
	"errors"
	"strconv"
	"strings"
)

func TCToSeconds(tc string) (float64, error) {
	splits := strings.Split(tc, "@")
	if len(splits) != 2 {
		return 0, errors.New("Invalid timecode: " + tc)
	}

	samples, err := strconv.ParseFloat(splits[0], 64)
	if err != nil {
		return 0, err
	}

	if splits[1] != "PAL" {
		return 0, errors.New("Invalid timecode. Currently only <NUMBER>@PAL is supported: " + tc)
	}

	// PAL = 25 fps
	// http://10.12.128.15:8080/APIdoc/time.html#time-bases
	return samples / 25, nil
}
