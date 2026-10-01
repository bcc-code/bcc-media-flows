package transcode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseSRT(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []srtCue
	}{
		{
			name: "multi-line text",
			in:   "1\n00:00:01,000 --> 00:00:02,500\nfirst\nsecond\n\n2\n01:02:03,004 --> 01:02:04,000\nthird\n\n",
			want: []srtCue{
				{Start: 1000, End: 2500, Text: "first\nsecond"},
				{Start: 3723004, End: 3724000, Text: "third"},
			},
		},
		{
			name: "BOM and CRLF",
			in:   "\ufeff1\r\n00:00:01,000 --> 00:00:02,000\r\nhello\r\n\r\n",
			want: []srtCue{{Start: 1000, End: 2000, Text: "hello"}},
		},
		{
			name: "dot before milliseconds",
			in:   "1\n00:00:01.250 --> 00:00:02.750\nhello\n\n",
			want: []srtCue{{Start: 1250, End: 2750, Text: "hello"}},
		},
		{
			name: "last block without blank line",
			in:   "1\n00:00:01,000 --> 00:00:02,000\nhello",
			want: []srtCue{{Start: 1000, End: 2000, Text: "hello"}},
		},
		{
			name: "end before start is kept",
			in:   "1\n00:26:19,880 --> 00:26:11,200\nwhisper\n\n",
			want: []srtCue{{Start: 1579880, End: 1571200, Text: "whisper"}},
		},
		{
			name: "empty",
			in:   "",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSRT([]byte(tt.in))
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_parseSRT_InvalidTimestamp(t *testing.T) {
	_, err := parseSRT([]byte("1\n00:00:xx,000 --> 00:00:02,000\nhello\n\n"))
	assert.ErrorIs(t, err, errInvalidSRTTimestamp)
}

func Test_repairBackwardCues(t *testing.T) {
	cues := []srtCue{
		{Start: 1000, End: 500},
		{Start: 2000, End: 3000},
		{Start: 4000, End: 100},
	}

	repairBackwardCues(cues)

	assert.Equal(t, []srtCue{
		{Start: 1000, End: 2000},
		{Start: 2000, End: 3000},
		{Start: 4000, End: 4000},
	}, cues)
}

func Test_secondsToMillis(t *testing.T) {
	assert.Equal(t, int64(188740), secondsToMillis(1522.74-1334))
	assert.Equal(t, int64(6717960), secondsToMillis(6717.96))
}
