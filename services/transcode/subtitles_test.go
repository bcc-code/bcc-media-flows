package transcode

import (
	"os"
	"testing"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/stretchr/testify/assert"
)

func Test_SubtitlesBurnIn(t *testing.T) {
	t.Skip("TODO: setup test data")
	videoPath := paths.Path{
		Drive: paths.TempDrive,
		Path:  "out.mp4",
	}
	subtitlePath := paths.Path{
		Drive: paths.TempDrive,
		Path:  "out.srt",
	}

	outputPath := paths.Path{
		Drive: paths.TempDrive,
		Path:  "",
	}

	subtitleHeaderPath := paths.Path{
		Drive: paths.TempDrive,
		Path:  "header.aas",
	}

	p, err := SubtitleBurnIn(videoPath, subtitlePath, subtitleHeaderPath, outputPath, func(progress ffmpeg.Progress) {
		t.Logf("Progress: %v", progress.Percent)
	})
	assert.Nil(t, err)
	assert.NotNil(t, p)
}

func Test_convertTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"zero", "0:00:00.000", "0:00:00.00"},
		{"simple", "0:01:23.456", "0:01:23.46"},
		{"leading zero hours", "0:00:05.100", "0:00:05.10"},
		{"large hours", "2:30:45.678", "2:30:45.68"},
		{"single digit hour", "1:00:00.000", "1:00:00.00"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := convertTimestamp(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func Test_convertTimeFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"zero", "00:00:00,000", "0:00:00.00"},
		{"typical", "00:01:23,456", "0:01:23.46"},
		{"hour", "01:30:45,678", "1:30:45.68"},
		{"high ms", "00:00:00,999", "0:00:00.00"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := convertTimeFormat(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func Test_writeEvent_SingleLine(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "subtitle_test_*.ass")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	writeEvent(tmpFile, "00:00:01,000", "00:00:05,000", []string{"Hello world"}, 0.00011)

	content, err := os.ReadFile(tmpFile.Name())
	assert.NoError(t, err)

	assert.Contains(t, string(content), "Dialogue: 0,")
	assert.Contains(t, string(content), "Hello world")
}

func Test_writeEvent_TwoLines(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "subtitle_test_*.ass")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	writeEvent(tmpFile, "00:00:01,000", "00:00:05,000", []string{"Line one", "Line two"}, 0.00011)

	content, err := os.ReadFile(tmpFile.Name())
	assert.NoError(t, err)

	line := string(content)
	assert.Contains(t, line, `\org(-2000000,0)`)
	assert.Contains(t, line, "Line one")
	assert.Contains(t, line, `\N`)
	assert.Contains(t, line, "Line two")
}

func Test_writeEvent_ThreeLines(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "subtitle_test_*.ass")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	writeEvent(tmpFile, "00:00:01,000", "00:00:05,000", []string{"A", "B", "C"}, 0.00011)

	content, err := os.ReadFile(tmpFile.Name())
	assert.NoError(t, err)

	line := string(content)
	assert.Contains(t, line, `A\NB\NC`)
}
