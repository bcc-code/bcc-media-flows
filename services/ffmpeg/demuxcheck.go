package ffmpeg

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DemuxCheckMaxMessages caps how many ffmpeg messages a result carries. A
	// badly damaged file logs one line per corrupt packet, which for a long
	// master runs to millions; the count is kept, the tail is dropped.
	DemuxCheckMaxMessages = 200

	// demuxCheckMaxMessageLength bounds a single message, in case ffmpeg dumps
	// a whole packet.
	demuxCheckMaxMessageLength = 500

	// demuxCheckShortReadTolerance is how far the processed duration may fall
	// short of the probed one before it counts as a truncated file. Edit lists
	// and audio priming make the two differ slightly on a healthy file.
	demuxCheckShortReadTolerance = time.Second
	demuxCheckShortReadFraction  = 0.02

	// demuxCheckScanBuffer is the longest stderr line accepted. ffmpeg lines are
	// short, but a runaway one must not abort the scan.
	demuxCheckScanBuffer = 1024 * 1024
)

// DemuxCheckMessage is one warning or error ffmpeg logged while reading the
// file.
type DemuxCheckMessage struct {
	// Level is warning, error, fatal or panic.
	Level string
	// Component is what logged it, such as "mov,mp4,m4a,3gp,3g2,mj2" or "h264",
	// with ffmpeg's pointer suffix removed.
	Component string
	Message   string
}

// DemuxCheckResult is what a full read of the file produced. It is a result
// even when the file is broken: only a check that could not run is an error.
type DemuxCheckResult struct {
	Command string

	// Errors counts error, fatal and panic messages plus the synthetic short
	// read finding; Warnings counts warning messages.
	Errors   int
	Warnings int

	// Messages holds the first DemuxCheckMaxMessages messages in log order;
	// TotalMessages is how many there were.
	Messages      []DemuxCheckMessage
	TotalMessages int

	// ExitError is set when ffmpeg exited non-zero, which it does for a file it
	// cannot open at all. It is empty for a file it could read to the end,
	// however many errors it logged on the way.
	ExitError string

	// ProcessedSeconds is how far into the file ffmpeg got, from its progress
	// output; TotalSeconds is the probed duration. A processed duration well
	// short of the total means the file ends before its header says it does.
	ProcessedSeconds float64
	TotalSeconds     float64
	// ShortRead is true when that shortfall exceeds the tolerance.
	ShortRead bool

	Duration time.Duration
}

// Passed is true when nothing at error level was found.
func (r DemuxCheckResult) Passed() bool {
	return r.Errors == 0 && r.ExitError == ""
}

// DemuxCheckArguments is the command line the check runs. Every stream is
// copied into the null muxer, so the whole file is demuxed and parsed but
// nothing is decoded.
func DemuxCheckArguments(path string) []string {
	return []string{
		"-hide_banner", "-nostdin", "-nostats",
		// level tags each line with its severity so it can be classified;
		// repeat keeps ffmpeg from collapsing identical lines, so the count is real.
		"-loglevel", "repeat+level+warning",
		"-progress", "pipe:1",
		"-i", path,
		"-map", "0",
		"-c", "copy",
		"-ignore_unknown",
		"-f", "null", "-",
	}
}

// DemuxCheck reads the whole file through ffmpeg and collects what it complains
// about. info is the probe result used for progress and the short read check;
// it is probed when nil.
//
// An error is returned only when the check itself could not run: ffmpeg
// missing, the file unreadable before ffmpeg started. ffmpeg failing on the
// file is a result, not an error.
func DemuxCheck(path string, info *StreamInfo, cb ProgressCallback) (DemuxCheckResult, error) {
	if path == "" {
		return DemuxCheckResult{}, errors.New("demux check has no input")
	}

	streamInfo := StreamInfo{}
	if info != nil {
		streamInfo = *info
	} else if probed, err := ProbeFile(path); err == nil && len(probed.Streams) > 0 {
		// A file ffprobe cannot read, or finds no streams in, still gets the
		// ffmpeg pass, whose messages say why. The short read check is skipped
		// for it because there is no duration to compare against.
		streamInfo = ProbeResultToInfo(probed)
	}

	args := DemuxCheckArguments(path)
	cmd := exec.Command("ffmpeg", args...)

	result := DemuxCheckResult{
		Command:      "ffmpeg " + strings.Join(args, " "),
		TotalSeconds: streamInfo.TotalSeconds,
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result, fmt.Errorf("could not open stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return result, fmt.Errorf("could not open stderr pipe: %w", err)
	}

	if cb != nil {
		cb(Progress{Params: strings.Join(args, " ")})
	}

	started := time.Now()
	if err := cmd.Start(); err != nil {
		return result, fmt.Errorf("starting ffmpeg: %w", err)
	}

	// Both pipes are drained concurrently. A damaged file produces stderr
	// faster than anything else, and a pipe nobody reads blocks ffmpeg.
	collector := newDemuxMessageCollector()
	var wg sync.WaitGroup
	var stderrErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		stderrErr = collector.consume(stderr)
	}()

	onProgress := parseProgressCallback(args, streamInfo, cb)
	var processedMicros float64
	stdoutScanner := newDemuxLineScanner(stdout)
	for stdoutScanner.Scan() {
		line := stdoutScanner.Text()
		if value, ok := strings.CutPrefix(line, "out_time_us="); ok {
			if micros, err := strconv.ParseFloat(value, 64); err == nil && micros > processedMicros {
				processedMicros = micros
			}
		}
		onProgress(line)
	}
	stdoutErr := stdoutScanner.Err()

	waitErr := cmd.Wait()
	wg.Wait()
	result.Duration = time.Since(started)

	if stdoutErr != nil {
		return result, fmt.Errorf("reading ffmpeg progress: %w", stdoutErr)
	}
	if stderrErr != nil {
		return result, fmt.Errorf("reading ffmpeg log: %w", stderrErr)
	}

	result.Messages = collector.messages
	result.TotalMessages = collector.total
	result.Errors = collector.errors
	result.Warnings = collector.warnings
	result.ProcessedSeconds = processedMicros / 1e6

	if waitErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(waitErr, &exitErr) {
			return result, fmt.Errorf("waiting for ffmpeg: %w", waitErr)
		}
		result.ExitError = fmt.Sprintf("ffmpeg exited with %d", exitErr.ExitCode())
		if last := collector.lastError; last != "" {
			result.ExitError += ": " + last
		}
	}

	if result.TotalSeconds > 0 && result.ExitError == "" {
		missing := result.TotalSeconds - result.ProcessedSeconds
		tolerance := math.Max(demuxCheckShortReadTolerance.Seconds(), result.TotalSeconds*demuxCheckShortReadFraction)
		if missing > tolerance {
			result.ShortRead = true
			result.Errors++
			result.TotalMessages++
			if len(result.Messages) < DemuxCheckMaxMessages {
				result.Messages = append(result.Messages, DemuxCheckMessage{
					Level:     "error",
					Component: "demux check",
					Message: fmt.Sprintf("Reading stopped at %s of the %s the file reports; the file is shorter than its header says",
						formatSeconds(result.ProcessedSeconds), formatSeconds(result.TotalSeconds)),
				})
			}
		}
	}

	return result, nil
}

// demuxLogLine matches one ffmpeg log line with the level flag on: an optional
// "[component @ 0xaddr] " prefix, the "[level] " tag, then the message. Lines
// without a level tag are continuations of the previous message.
// demuxCorruptionWarning matches the warnings ffmpeg logs for damaged essence.
// ffmpeg flags a corrupt packet and carries on, so it logs these at warning
// level; for a file about to be archived they are errors, and a damaged MXF
// or a truncated one would otherwise pass with warnings.
var demuxCorruptionWarning = regexp.MustCompile(`(?i)corrupt|sync lost|partial file|truncat`)

var demuxLogLine = regexp.MustCompile(`^(?:\[([^\]]*?) @ 0x[0-9a-fA-F]+\] )?\[(warning|error|fatal|panic|info|verbose|debug|trace)\] (.*)$`)

type demuxMessageCollector struct {
	messages  []DemuxCheckMessage
	total     int
	errors    int
	warnings  int
	lastError string
	// lastKept is true while the previous line went into messages, so a
	// continuation line knows whether it has anything to attach to.
	lastKept bool
}

func newDemuxMessageCollector() *demuxMessageCollector {
	return &demuxMessageCollector{}
}

func (c *demuxMessageCollector) consume(r io.Reader) error {
	scanner := newDemuxLineScanner(r)
	for scanner.Scan() {
		c.add(scanner.Text())
	}
	return scanner.Err()
}

func (c *demuxMessageCollector) add(line string) {
	line = strings.TrimRight(line, "\r")
	if strings.TrimSpace(line) == "" {
		return
	}

	match := demuxLogLine.FindStringSubmatch(line)
	if match == nil {
		if c.lastKept {
			last := &c.messages[len(c.messages)-1]
			last.Message = truncateMessage(last.Message + " " + strings.TrimSpace(line))
		}
		return
	}

	level, component, message := match[2], match[1], truncateMessage(match[3])
	if level == "warning" && demuxCorruptionWarning.MatchString(message) {
		level = "error"
	}
	switch level {
	case "error", "fatal", "panic":
		c.errors++
		c.lastError = message
	case "warning":
		c.warnings++
	default:
		// Not asked for at this loglevel, but harmless if it turns up.
		c.lastKept = false
		return
	}

	c.total++
	c.lastKept = len(c.messages) < DemuxCheckMaxMessages
	if c.lastKept {
		c.messages = append(c.messages, DemuxCheckMessage{
			Level:     level,
			Component: component,
			Message:   message,
		})
	}
}

func truncateMessage(s string) string {
	if len(s) <= demuxCheckMaxMessageLength {
		return s
	}
	return s[:demuxCheckMaxMessageLength] + "…"
}

func newDemuxLineScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), demuxCheckScanBuffer)
	return scanner
}

func formatSeconds(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second)).Round(10 * time.Millisecond)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := d.Seconds() - float64(h*3600+m*60)
	return fmt.Sprintf("%02d:%02d:%05.2f", h, m, s)
}
