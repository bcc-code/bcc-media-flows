package export

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createMetadataResult(id, title string) *vsapi.MetadataResult {
	return &vsapi.MetadataResult{
		ID: id,
		Terse: map[string][]*vsapi.MetadataField{
			vscommon.FieldTitle.Value: {{
				Value: title,
			}},
		},
	}
}

func createRow(label, editorialStatus, status string) *ShortsData {
	return &ShortsData{
		Label:           label,
		EditorialStatus: editorialStatus,
		Status:          status,
	}
}

func TestMapAndFilterShortsData(t *testing.T) {
	tests := []struct {
		name          string
		data          []*ShortsData
		mbItems       []*vsapi.MetadataResult
		expectedLen   int
		expectedIDs   []string
		expectLabels  []string
		expectNoError bool
	}{
		{
			name: "basic matching",
			data: []*ShortsData{
				createRow("test1", "Ready in MB", ""),
				createRow("test2", "Ready in MB", ""),
				createRow("test3", "Not Ready", ""),
			},
			mbItems: []*vsapi.MetadataResult{
				createMetadataResult("id1", "test1"),
				createMetadataResult("id2", "test2"),
				createMetadataResult("id3", "test3"),
			},
			expectedLen:   2,
			expectedIDs:   []string{"id1", "id2"},
			expectLabels:  []string{"test1", "test2"},
			expectNoError: true,
		},
		{
			name: "no matches",
			data: []*ShortsData{
				createRow("test1", "Ready in MB", ""),
			},
			mbItems: []*vsapi.MetadataResult{
				createMetadataResult("id1", "different"),
			},
			expectedLen:   0,
			expectedIDs:   []string{},
			expectLabels:  []string{},
			expectNoError: true,
		},
		{
			name:          "empty inputs",
			data:          []*ShortsData{},
			mbItems:       []*vsapi.MetadataResult{},
			expectedLen:   0,
			expectedIDs:   []string{},
			expectLabels:  []string{},
			expectNoError: true,
		},
		{
			name: "title with dot suffix",
			data: []*ShortsData{
				createRow("test1", "Ready in MB", ""),
			},
			mbItems: []*vsapi.MetadataResult{
				createMetadataResult("id1", "test1.suffix"),
			},
			expectedLen:   1,
			expectedIDs:   []string{"id1"},
			expectLabels:  []string{"test1"},
			expectNoError: true,
		},
		{
			name: "filter out done status",
			data: []*ShortsData{
				createRow("test1", "Ready in MB", "Done"),
				createRow("test2", "Ready in MB", "In Progress"),
			},
			mbItems: []*vsapi.MetadataResult{
				createMetadataResult("id1", "test1"),
				createMetadataResult("id2", "test2"),
			},
			expectedLen:   1,
			expectedIDs:   []string{"id2"},
			expectLabels:  []string{"test2"},
			expectNoError: true,
		},
		{
			name: "empty status is allowed",
			data: []*ShortsData{
				createRow("test1", "Ready in MB", ""),
			},
			mbItems: []*vsapi.MetadataResult{
				createMetadataResult("id1", "test1"),
			},
			expectedLen:   1,
			expectedIDs:   []string{"id1"},
			expectLabels:  []string{"test1"},
			expectNoError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapAndFilterShortsData(tt.data, tt.mbItems)

			if tt.expectNoError {
				require.Len(t, result, tt.expectedLen, "unexpected result length")

				// Extract IDs and labels for comparison
				resultIDs := make([]string, 0, len(result))
				resultLabels := make([]string, 0, len(result))

				for _, item := range result {
					resultIDs = append(resultIDs, item.MBMetadata.ID)
					resultLabels = append(resultLabels, item.Label)
				}

				assert.ElementsMatch(t, tt.expectedIDs, resultIDs, "unexpected IDs in result")
				assert.ElementsMatch(t, tt.expectLabels, resultLabels, "unexpected labels in result")
			}
		})
	}
}

func TestConvertToSeconds(t *testing.T) {
	tests := []struct {
		input    string
		expected *int64
		hasError bool
	}{
		{"00:00:00", ptrInt64(0), false},
		{"12:34:56", ptrInt64(45296), false},
		{"5:00", ptrInt64(300), false},
		{"1:02:03", ptrInt64(3723), false},
		{"99:59:59", ptrInt64(359999), false},
		{"00:01:02", ptrInt64(62), false},
		{"bad", nil, true},
		{"12:34", ptrInt64(754), false},
		{"1:2:3", ptrInt64(3723), false},
		{"1", nil, true},
		{"", nil, true},
		{"12:34:56:78", nil, true},
	}

	for _, tc := range tests {
		result, err := convertToSeconds(tc.input)
		if tc.hasError {
			assert.Error(t, err, "input: %s", tc.input)
			assert.Nil(t, result, "input: %s", tc.input)
		} else {
			assert.NoError(t, err, "input: %s", tc.input)
			assert.NotNil(t, result, "input: %s", tc.input)
			assert.Equal(t, *tc.expected, *result, "input: %s", tc.input)
		}
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}
