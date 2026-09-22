package ffmpeg

import (
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateDemuxTestFile writes a short MXF with video and 48 kHz audio, which is
// what a master looks like.
func generateDemuxTestFile(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	path := filepath.Join(t.TempDir(), "good.mxf")
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=3:size=320x240:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=3:sample_rate=48000",
		"-c:v", "mpeg2video", "-c:a", "pcm_s16le", path)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return path
}

func TestDemuxCheck_GoodFilePasses(t *testing.T) {
	path := generateDemuxTestFile(t)

	var progress []Progress
	result, err := DemuxCheck(path, nil, func(p Progress) { progress = append(progress, p) })
	require.NoError(t, err)

	assert.True(t, result.Passed(), "messages: %+v", result.Messages)
	assert.Equal(t, 0, result.Errors)
	assert.Empty(t, result.ExitError)
	assert.False(t, result.ShortRead)
	assert.InDelta(t, 3, result.TotalSeconds, 0.1)
	assert.InDelta(t, 3, result.ProcessedSeconds, 0.1)
	assert.NotContains(t, result.Command, "-c copy")
	assert.NotEmpty(t, progress, "progress is what keeps the activity heartbeat alive")
}

func TestDemuxCheck_TruncatedFileIsAShortRead(t *testing.T) {
	good := generateDemuxTestFile(t)
	data, err := os.ReadFile(good)
	require.NoError(t, err)

	truncated := filepath.Join(t.TempDir(), "truncated.mxf")
	require.NoError(t, os.WriteFile(truncated, data[:len(data)*6/10], 0o644))

	result, err := DemuxCheck(truncated, nil, nil)
	require.NoError(t, err)

	assert.True(t, result.ShortRead, "read %.2f of %.2f", result.ProcessedSeconds, result.TotalSeconds)
	assert.False(t, result.Passed())
	assert.GreaterOrEqual(t, result.Errors, 1)
	assert.Less(t, result.ProcessedSeconds, result.TotalSeconds-1)

	last := result.Messages[len(result.Messages)-1]
	assert.Equal(t, "demux check", last.Component)
	assert.Contains(t, last.Message, "shorter than its header says")
}

func TestDemuxCheck_CorruptedPacketsAreErrors(t *testing.T) {
	good := generateDemuxTestFile(t)
	data, err := os.ReadFile(good)
	require.NoError(t, err)

	// Overwrite a stretch in the middle of the essence, well past the header.
	garbage := make([]byte, 20000)
	_, err = rand.Read(garbage)
	require.NoError(t, err)
	copy(data[len(data)/2:], garbage)

	corrupt := filepath.Join(t.TempDir(), "corrupt.mxf")
	require.NoError(t, os.WriteFile(corrupt, data, 0o644))

	result, err := DemuxCheck(corrupt, nil, nil)
	require.NoError(t, err)

	assert.False(t, result.Passed(), "messages: %+v", result.Messages)
	assert.Greater(t, result.Errors+result.Warnings, 0)
	assert.Equal(t, result.TotalMessages, len(result.Messages))
	for _, m := range result.Messages {
		assert.NotContains(t, m.Component, "0x", "pointer suffix should be stripped")
	}
}

// generateDemuxProResTestFile writes a short ProRes .mov, which is what a
// Resolve export looks like. Its frames are self-contained, so damage inside
// one leaves the container intact and only the decoder notices.
func generateDemuxProResTestFile(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	path := filepath.Join(t.TempDir(), "good.mov")
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=3:size=320x240:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=3:sample_rate=48000",
		"-c:v", "prores_ks", "-profile:v", "3", "-c:a", "pcm_s24le", path)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return path
}

func TestDemuxCheck_GoodProResFilePasses(t *testing.T) {
	path := generateDemuxProResTestFile(t)

	result, err := DemuxCheck(path, nil, nil)
	require.NoError(t, err)

	assert.True(t, result.Passed(), "messages: %+v", result.Messages)
	assert.False(t, result.ShortRead)
}

// A frame with a valid packet but garbage inside it. The mov demuxer hands it
// over without a word; the check has to decode to see it.
func TestDemuxCheck_CorruptProResFramesAreErrors(t *testing.T) {
	good := generateDemuxProResTestFile(t)
	data, err := os.ReadFile(good)
	require.NoError(t, err)

	garbage := make([]byte, 20000)
	_, err = rand.Read(garbage)
	require.NoError(t, err)
	copy(data[len(data)/2:], garbage)

	corrupt := filepath.Join(t.TempDir(), "corrupt.mov")
	require.NoError(t, os.WriteFile(corrupt, data, 0o644))

	result, err := DemuxCheck(corrupt, nil, nil)
	require.NoError(t, err)

	assert.False(t, result.Passed(), "messages: %+v", result.Messages)
	assert.Greater(t, result.Errors, 0)
	assert.Empty(t, result.ExitError, "ffmpeg reads past a bad frame, so this is a verdict from the log, not an exit code")

	var fromDecoder bool
	for _, m := range result.Messages {
		if strings.Contains(m.Component, "prores") {
			fromDecoder = true
			break
		}
	}
	assert.True(t, fromDecoder, "expected a prores decoder message, got %+v", result.Messages)
}

func TestDemuxCheck_UnreadableFileIsAVerdictNotAnError(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	path := filepath.Join(t.TempDir(), "not-media.mxf")
	require.NoError(t, os.WriteFile(path, []byte("this is not an mxf file"), 0o644))

	result, err := DemuxCheck(path, nil, nil)
	require.NoError(t, err)

	assert.False(t, result.Passed())
	assert.NotEmpty(t, result.ExitError)
	assert.False(t, result.ShortRead, "no duration is known for a file ffmpeg cannot open")
}

func TestDemuxCheck_NoInput(t *testing.T) {
	_, err := DemuxCheck("", nil, nil)
	assert.Error(t, err)
}

func TestDemuxMessageCollector_ParsesLevelsAndStripsPointers(t *testing.T) {
	c := newDemuxMessageCollector()
	for _, line := range []string{
		"[mov,mp4,m4a,3gp,3g2,mj2 @ 0x7f8b1c008200] [warning] stream 0, offset 0x1234: partial file",
		"[h264 @ 0xa9f05d500] [error] Invalid NAL unit size (351865195 > 58).",
		"[in#0 @ 0xb89020000] [warning] broken or empty index",
		"[error] Error opening input file /nope.mov.",
		"    continuation of the previous line",
		"[info] not asked for, but harmless",
		"",
		"[fatal] Conversion failed!",
	} {
		c.add(line)
	}

	assert.Equal(t, 5, c.total)
	assert.Equal(t, 4, c.errors, "a partial file is corruption, so its warning counts as an error")
	assert.Equal(t, 1, c.warnings)
	assert.Equal(t, "Conversion failed!", c.lastError)

	require.Len(t, c.messages, 5)
	assert.Equal(t, DemuxCheckMessage{Level: "error", Component: "mov,mp4,m4a,3gp,3g2,mj2", Message: "stream 0, offset 0x1234: partial file"}, c.messages[0])
	assert.Equal(t, DemuxCheckMessage{Level: "error", Component: "h264", Message: "Invalid NAL unit size (351865195 > 58)."}, c.messages[1])
	assert.Equal(t, "in#0", c.messages[2].Component)
	assert.Equal(t, DemuxCheckMessage{Level: "error", Message: "Error opening input file /nope.mov. continuation of the previous line"}, c.messages[3])
	assert.Equal(t, "fatal", c.messages[4].Level)
}

// Decoder lines carry two prefixes: the input stream and the decoder. These
// are verbatim from ffmpeg 9 reading a damaged ProRes master.
func TestDemuxMessageCollector_ParsesDecoderLinesWithTwoPrefixes(t *testing.T) {
	c := newDemuxMessageCollector()
	c.add("[vist#0:0/prores @ 0x94f014300] [dec:prores @ 0x94f01c280] [error] Error submitting packet to decoder: Invalid data found when processing input")
	c.add("[vist#0:0/prores @ 0x94f014300] [dec:prores @ 0x94f01c280] [warning] corrupt decoded frame")
	c.add("[prores @ 0x94f07ca80] [error] invalid frame header")

	assert.Equal(t, 3, c.total)
	assert.Equal(t, 3, c.errors, "a corrupt decoded frame is corruption, so its warning counts as an error")
	assert.Equal(t, 0, c.warnings)

	require.Len(t, c.messages, 3)
	assert.Equal(t, DemuxCheckMessage{
		Level:     "error",
		Component: "vist#0:0/prores dec:prores",
		Message:   "Error submitting packet to decoder: Invalid data found when processing input",
	}, c.messages[0])
	assert.Equal(t, DemuxCheckMessage{Level: "error", Component: "vist#0:0/prores dec:prores", Message: "corrupt decoded frame"}, c.messages[1])
	assert.Equal(t, DemuxCheckMessage{Level: "error", Component: "prores", Message: "invalid frame header"}, c.messages[2])
}

func TestDemuxMessageCollector_PromotesCorruptionWarnings(t *testing.T) {
	c := newDemuxMessageCollector()
	c.add("[in#0/mxf @ 0x1] [warning] edit unit sync lost on stream 0, jumping from 37 to 40")
	c.add("[in#0/mxf @ 0x1] [warning] Packet corrupt (stream = 1, dts = NOPTS).")
	c.add("[aist#0:1/pcm_s16le @ 0x1] [warning] Guessed Channel Layout: mono")

	assert.Equal(t, 2, c.errors)
	assert.Equal(t, 1, c.warnings)
	assert.Equal(t, "error", c.messages[0].Level)
	assert.Equal(t, "error", c.messages[1].Level)
	assert.Equal(t, "warning", c.messages[2].Level)
}

func TestDemuxMessageCollector_CapsMessagesButKeepsCounting(t *testing.T) {
	c := newDemuxMessageCollector()
	for i := 0; i < DemuxCheckMaxMessages+25; i++ {
		c.add("[h264 @ 0x1] [error] missing picture in access unit")
	}
	// A continuation after the cap has nothing to attach to and must not panic.
	c.add("    trailing detail")

	assert.Equal(t, DemuxCheckMaxMessages+25, c.total)
	assert.Equal(t, DemuxCheckMaxMessages+25, c.errors)
	assert.Len(t, c.messages, DemuxCheckMaxMessages)
	assert.False(t, strings.Contains(c.messages[len(c.messages)-1].Message, "trailing detail"))
}

func TestDemuxCheckArguments(t *testing.T) {
	args := strings.Join(DemuxCheckArguments("/in.mov"), " ")
	assert.Contains(t, args, "-loglevel repeat+level+warning")
	assert.Contains(t, args, "-i /in.mov -map 0")
	assert.NotContains(t, args, "-c copy", "the check must decode, or damage inside a frame passes")
	assert.True(t, strings.HasSuffix(args, "-f null -"))
}

func TestFormatSeconds(t *testing.T) {
	assert.Equal(t, "00:00:03.00", formatSeconds(3))
	assert.Equal(t, "01:02:03.50", formatSeconds(3723.5))
}
