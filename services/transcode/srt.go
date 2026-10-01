package transcode

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var errInvalidSRTTimestamp = errors.New("invalid SRT timestamp")

// srtCue is one subtitle entry. Times are in milliseconds so that shifting and
// comparing them is exact.
type srtCue struct {
	Start int64
	End   int64
	Text  string
}

// secondsToMillis rounds rather than truncates, so a value like 188.74 that
// floats as 188.73999 does not lose a millisecond.
func secondsToMillis(seconds float64) int64 {
	return int64(math.Round(seconds * 1000))
}

// parseSRT reads the cues of an SRT file. The index lines are ignored, since
// the cues are renumbered when written. Cues are returned in file order, and
// cues ending before they start are kept as they are: Whisper produces them.
func parseSRT(data []byte) ([]srtCue, error) {
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	text := strings.ReplaceAll(string(data), "\r\n", "\n")

	var cues []srtCue
	for _, block := range strings.Split(text, "\n\n") {
		lines := strings.Split(strings.Trim(block, "\n"), "\n")

		timingLine := -1
		for i, line := range lines {
			if strings.Contains(line, "-->") {
				timingLine = i
				break
			}
		}
		if timingLine == -1 {
			// Blank or stray blocks carry no cue.
			continue
		}

		start, end, err := parseSRTTiming(lines[timingLine])
		if err != nil {
			return nil, err
		}

		cues = append(cues, srtCue{
			Start: start,
			End:   end,
			Text:  strings.Join(lines[timingLine+1:], "\n"),
		})
	}

	return cues, nil
}

func parseSRTTiming(line string) (int64, int64, error) {
	parts := strings.SplitN(line, "-->", 2)
	start, err := parseSRTTimestamp(parts[0])
	if err != nil {
		return 0, 0, err
	}

	// Anything after the end time, such as position hints, is dropped.
	endFields := strings.Fields(parts[1])
	if len(endFields) == 0 {
		return 0, 0, fmt.Errorf("%w: missing end in %q", errInvalidSRTTimestamp, line)
	}
	end, err := parseSRTTimestamp(endFields[0])
	if err != nil {
		return 0, 0, err
	}

	return start, end, nil
}

// parseSRTTimestamp parses HH:MM:SS,mmm, also accepting a dot before the
// milliseconds.
func parseSRTTimestamp(ts string) (int64, error) {
	ts = strings.TrimSpace(ts)
	clock, millis, ok := strings.Cut(strings.Replace(ts, ".", ",", 1), ",")
	hms := strings.Split(clock, ":")
	if !ok || len(hms) != 3 {
		return 0, fmt.Errorf("%w: %q", errInvalidSRTTimestamp, ts)
	}

	var total int64
	for _, part := range hms {
		v, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: %q: %w", errInvalidSRTTimestamp, ts, err)
		}
		total = total*60 + v
	}

	ms, err := strconv.ParseInt(millis, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q: %w", errInvalidSRTTimestamp, ts, err)
	}

	return total*1000 + ms, nil
}

// repairBackwardCues gives a cue that ends before it starts the end ffmpeg
// gives it: the start of the next cue, or its own start if it is the last one.
// cues must be sorted by start.
func repairBackwardCues(cues []srtCue) {
	for i := range cues {
		if cues[i].End >= cues[i].Start {
			continue
		}
		if i+1 < len(cues) {
			cues[i].End = cues[i+1].Start
		} else {
			cues[i].End = cues[i].Start
		}
	}
}

func formatSRTTimestamp(ms int64) string {
	return fmt.Sprintf("%02d:%02d:%02d,%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}

// writeSRT renders the cues, numbered from 1, in the layout ffmpeg writes.
func writeSRT(cues []srtCue) []byte {
	var b strings.Builder
	for i, cue := range cues {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, formatSRTTimestamp(cue.Start), formatSRTTimestamp(cue.End), cue.Text)
	}
	return []byte(b.String())
}
