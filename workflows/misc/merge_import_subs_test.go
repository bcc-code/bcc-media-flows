package miscworkflows

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSubMergeData(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		separator rune
		want      []SubtitleEntry
	}{
		{
			name:      "comma separated",
			input:     "Subtrans ID,Timecode start\n123,00:00:01:500\n456,00:01:02:250\n",
			separator: ',',
			want: []SubtitleEntry{
				{SubtransID: "123", TimecodeStr: "00:00:01:500"},
				{SubtransID: "456", TimecodeStr: "00:01:02:250"},
			},
		},
		{
			name:      "semicolon separated",
			input:     "Subtrans ID;Timecode start\n123;00:00:01:500\n",
			separator: ';',
			want:      []SubtitleEntry{{SubtransID: "123", TimecodeStr: "00:00:01:500"}},
		},
		{
			name:      "columns matched by name, not position",
			input:     "Title,Timecode start,Subtrans ID\nsomething,00:00:01:500,123\n",
			separator: ',',
			want:      []SubtitleEntry{{SubtransID: "123", TimecodeStr: "00:00:01:500"}},
		},
		{
			name:      "header only",
			input:     "Subtrans ID,Timecode start\n",
			separator: ',',
			want:      []SubtitleEntry{},
		},
		{
			name:      "empty input",
			input:     "",
			separator: ',',
			want:      nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseSubMergeData([]byte(test.input), test.separator)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestParseSubMergeDataRejectsMissingColumns(t *testing.T) {
	_, err := parseSubMergeData([]byte("Subtrans ID,Timecode\n123,00:00:01:500\n"), ',')
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Timecode start")
}

// The parser used to configure gocsv through a package-level global, so two
// workflows parsing with different separators could pick up each other's.
func TestParseSubMergeDataSeparatorIsNotShared(t *testing.T) {
	const rows = 200

	comma := "Subtrans ID,Timecode start\n" + strings.Repeat("123,00:00:01:500\n", rows)
	semicolon := "Subtrans ID;Timecode start\n" + strings.Repeat("456;00:00:02:500\n", rows)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			got, err := parseSubMergeData([]byte(comma), ',')
			assert.NoError(t, err)
			assert.Len(t, got, rows)
			if len(got) > 0 {
				assert.Equal(t, "123", got[0].SubtransID)
			}
		}()
		go func() {
			defer wg.Done()
			got, err := parseSubMergeData([]byte(semicolon), ';')
			assert.NoError(t, err)
			assert.Len(t, got, rows)
			if len(got) > 0 {
				assert.Equal(t, "456", got[0].SubtransID)
			}
		}()
	}
	wg.Wait()
}

func TestGetSeparatorRune(t *testing.T) {
	assert.Equal(t, ';', getSeparatorRune(";"))
	assert.Equal(t, ',', getSeparatorRune(""))
	assert.Equal(t, '\t', getSeparatorRune("\t"))
	assert.Equal(t, ';', getSeparatorRune(";extra"))
}
