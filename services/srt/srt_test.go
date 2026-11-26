package srt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []Cue
	}{
		{
			name: "multi-line text",
			in:   "1\n00:00:01,000 --> 00:00:02,500\nfirst\nsecond\n\n2\n01:02:03,004 --> 01:02:04,000\nthird\n\n",
			want: []Cue{
				{Start: 1000, End: 2500, Text: "first\nsecond"},
				{Start: 3723004, End: 3724000, Text: "third"},
			},
		},
		{
			name: "BOM and CRLF",
			in:   "\ufeff1\r\n00:00:01,000 --> 00:00:02,000\r\nhello\r\n\r\n",
			want: []Cue{{Start: 1000, End: 2000, Text: "hello"}},
		},
		{
			name: "dot before milliseconds",
			in:   "1\n00:00:01.250 --> 00:00:02.750\nhello\n\n",
			want: []Cue{{Start: 1250, End: 2750, Text: "hello"}},
		},
		{
			name: "last block without blank line",
			in:   "1\n00:00:01,000 --> 00:00:02,000\nhello",
			want: []Cue{{Start: 1000, End: 2000, Text: "hello"}},
		},
		{
			name: "end before start is kept",
			in:   "1\n00:26:19,880 --> 00:26:11,200\nwhisper\n\n",
			want: []Cue{{Start: 1579880, End: 1571200, Text: "whisper"}},
		},
		{
			name: "empty",
			in:   "",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.in))
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_parseSRT_InvalidTimestamp(t *testing.T) {
	_, err := Parse([]byte("1\n00:00:xx,000 --> 00:00:02,000\nhello\n\n"))
	assert.ErrorIs(t, err, ErrInvalidTimestamp)
}

func TestRepairBackward(t *testing.T) {
	cues := []Cue{
		{Start: 1000, End: 500},
		{Start: 2000, End: 3000},
		{Start: 4000, End: 100},
	}

	RepairBackward(cues)

	assert.Equal(t, []Cue{
		{Start: 1000, End: 2000},
		{Start: 2000, End: 3000},
		{Start: 4000, End: 4000},
	}, cues)
}

func TestSecondsToMillis(t *testing.T) {
	assert.Equal(t, int64(188740), SecondsToMillis(1522.74-1334))
	assert.Equal(t, int64(6717960), SecondsToMillis(6717.96))
}

func TestSanitizeSRT(t *testing.T) {
	for _, test := range []struct {
		name    string
		in      string
		want    string
		changed int
	}{
		{
			name: "valid file is only renumbered",
			in:   "5\n00:00:01,000 --> 00:00:02,000\nHei\n\n6\n00:00:02,000 --> 00:00:03,000\nverden\n",
			want: "1\n00:00:01,000 --> 00:00:02,000\nHei\n\n2\n00:00:02,000 --> 00:00:03,000\nverden\n\n",
		},
		{
			name:    "zero length runs to the next cue",
			in:      "1\n00:00:01,000 --> 00:00:01,000\nHei\n\n2\n00:00:01,500 --> 00:00:03,000\nverden\n",
			want:    "1\n00:00:01,000 --> 00:00:01,500\nHei\n\n2\n00:00:01,500 --> 00:00:03,000\nverden\n\n",
			changed: 1,
		},
		{
			name:    "backward cue runs to the next cue",
			in:      "1\n00:00:02,000 --> 00:00:01,000\nHei\n\n2\n00:00:04,000 --> 00:00:05,000\nverden\n",
			want:    "1\n00:00:02,000 --> 00:00:04,000\nHei\n\n2\n00:00:04,000 --> 00:00:05,000\nverden\n\n",
			changed: 1,
		},
		{
			name:    "next cue starting at the same time gives the minimum",
			in:      "1\n00:00:01,000 --> 00:00:01,000\nHei\n\n2\n00:00:01,000 --> 00:00:03,000\nverden\n",
			want:    "1\n00:00:01,000 --> 00:00:02,000\nHei\n\n2\n00:00:01,000 --> 00:00:03,000\nverden\n\n",
			changed: 1,
		},
		{
			name:    "last cue gets the minimum",
			in:      "1\n00:00:01,000 --> 00:00:02,000\nHei\n\n2\n00:00:09,000 --> 00:00:09,000\nverden\n",
			want:    "1\n00:00:01,000 --> 00:00:02,000\nHei\n\n2\n00:00:09,000 --> 00:00:10,000\nverden\n\n",
			changed: 1,
		},
		{
			name:    "empty cue is dropped",
			in:      "1\n00:00:01,000 --> 00:00:02,000\n \n\n2\n00:00:02,000 --> 00:00:03,000\nverden\n",
			want:    "1\n00:00:02,000 --> 00:00:03,000\nverden\n\n",
			changed: 1,
		},
		{
			name: "cues are ordered by start",
			in:   "1\n00:00:05,000 --> 00:00:06,000\nto\n\n2\n00:00:01,000 --> 00:00:02,000\nen\n",
			want: "1\n00:00:01,000 --> 00:00:02,000\nen\n\n2\n00:00:05,000 --> 00:00:06,000\nto\n\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, changed, err := Sanitize([]byte(test.in))
			require.NoError(t, err)
			assert.Equal(t, test.want, string(got))
			assert.Equal(t, test.changed, changed)
		})
	}

	_, _, err := Sanitize([]byte("1\nnot a time --> 00:00:01,000\nHei\n"))
	assert.ErrorIs(t, err, ErrInvalidTimestamp)
}

func TestRepair(t *testing.T) {
	cues := []Cue{
		{Start: 5000, End: 6000, Text: "to"},
		{Start: 3000, End: 4000, Text: "  "},
		{Start: 1000, End: 1000, Text: "en"},
		{Start: 9000, End: 8000, Text: "tre"},
	}

	got, changed := Repair(cues)

	assert.Equal(t, []Cue{
		{Start: 1000, End: 5000, Text: "en"},
		{Start: 5000, End: 6000, Text: "to"},
		{Start: 9000, End: 10000, Text: "tre"},
	}, got)
	assert.Equal(t, 3, changed, "one dropped, two repaired")
}
