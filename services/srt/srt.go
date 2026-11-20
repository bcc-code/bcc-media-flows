package srt

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

var ErrInvalidTimestamp = errors.New("invalid SRT timestamp")

// Cue is one subtitle entry. Times are in milliseconds so that shifting and
// comparing them is exact.
type Cue struct {
	Start int64
	End   int64
	Text  string
}

// SecondsToMillis rounds rather than truncates, so a value like 188.74 that
// floats as 188.73999 does not lose a millisecond.
func SecondsToMillis(seconds float64) int64 {
	return int64(math.Round(seconds * 1000))
}

// Parse reads the cues of an SRT file. The index lines are ignored, since
// the cues are renumbered when written. Cues are returned in file order, and
// cues ending before they start are kept as they are: Whisper produces them.
func Parse(data []byte) ([]Cue, error) {
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	text := strings.ReplaceAll(string(data), "\r\n", "\n")

	var cues []Cue
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

		start, end, err := parseTiming(lines[timingLine])
		if err != nil {
			return nil, err
		}

		cues = append(cues, Cue{
			Start: start,
			End:   end,
			Text:  strings.Join(lines[timingLine+1:], "\n"),
		})
	}

	return cues, nil
}

func parseTiming(line string) (int64, int64, error) {
	parts := strings.SplitN(line, "-->", 2)
	start, err := parseTimestamp(parts[0])
	if err != nil {
		return 0, 0, err
	}

	// Anything after the end time, such as position hints, is dropped.
	endFields := strings.Fields(parts[1])
	if len(endFields) == 0 {
		return 0, 0, fmt.Errorf("%w: missing end in %q", ErrInvalidTimestamp, line)
	}
	end, err := parseTimestamp(endFields[0])
	if err != nil {
		return 0, 0, err
	}

	return start, end, nil
}

// parseTimestamp parses HH:MM:SS,mmm, also accepting a dot before the
// milliseconds.
func parseTimestamp(ts string) (int64, error) {
	ts = strings.TrimSpace(ts)
	clock, millis, ok := strings.Cut(strings.Replace(ts, ".", ",", 1), ",")
	hms := strings.Split(clock, ":")
	if !ok || len(hms) != 3 {
		return 0, fmt.Errorf("%w: %q", ErrInvalidTimestamp, ts)
	}

	var total int64
	for _, part := range hms {
		v, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: %q: %w", ErrInvalidTimestamp, ts, err)
		}
		total = total*60 + v
	}

	ms, err := strconv.ParseInt(millis, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q: %w", ErrInvalidTimestamp, ts, err)
	}

	return total*1000 + ms, nil
}

// RepairBackward gives a cue that ends before it starts the end ffmpeg
// gives it: the start of the next cue, or its own start if it is the last one.
// cues must be sorted by start.
func RepairBackward(cues []Cue) {
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

func formatTimestamp(ms int64) string {
	return fmt.Sprintf("%02d:%02d:%02d,%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}

// Write renders the cues, numbered from 1, in the layout ffmpeg writes.
func Write(cues []Cue) []byte {
	var b strings.Builder
	for i, cue := range cues {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, formatTimestamp(cue.Start), formatTimestamp(cue.End), cue.Text)
	}
	return []byte(b.String())
}

// minRepairedCueMs is how long a cue with no length lasts when the next cue gives it none.
const minRepairedCueMs = 1000

// Sanitize makes a transcriber's SRT acceptable to strict readers such as Media
// Brain: cues without text are dropped, cues are ordered by start, and a cue that does
// not end after it starts runs until the next cue starts, or minRepairedCueMs when that
// gives it no length. No text is lost. It returns how many cues were dropped or repaired.
func Sanitize(data []byte) ([]byte, int, error) {
	cues, err := Parse(data)
	if err != nil {
		return nil, 0, err
	}

	changed := 0
	kept := cues[:0]
	for _, cue := range cues {
		if strings.TrimSpace(cue.Text) == "" {
			changed++
			continue
		}
		kept = append(kept, cue)
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Start < kept[j].Start })

	for i := range kept {
		if kept[i].End > kept[i].Start {
			continue
		}
		changed++
		if i+1 < len(kept) && kept[i+1].Start > kept[i].Start {
			kept[i].End = kept[i+1].Start
		} else {
			kept[i].End = kept[i].Start + minRepairedCueMs
		}
	}

	return Write(kept), changed, nil
}
