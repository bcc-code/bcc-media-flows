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

func createCSVRow(label, editorialStatus string) *ShortsCsvRow {
	return &ShortsCsvRow{
		Label:           label,
		EditorialStatus: editorialStatus,
		Status:          "", // Default empty status
	}
}

func createCSVRowWithStatus(label, editorialStatus, status string) *ShortsCsvRow {
	return &ShortsCsvRow{
		Label:           label,
		EditorialStatus: editorialStatus,
		Status:          status,
	}
}

func TestMapAndFilterShortsData(t *testing.T) {
	tests := []struct {
		name          string
		csvRows       []*ShortsCsvRow
		mbItems       []*vsapi.MetadataResult
		expectedLen   int
		expectedIDs   []string
		expectLabels  []string
		expectNoError bool
	}{
		{
			name: "basic matching",
			csvRows: []*ShortsCsvRow{
				createCSVRow("test1", "Ready in MB"),
				createCSVRow("test2", "Ready in MB"),
				createCSVRow("test3", "Not Ready"),
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
			csvRows: []*ShortsCsvRow{
				createCSVRow("test1", "Ready in MB"),
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
			csvRows:       []*ShortsCsvRow{},
			mbItems:       []*vsapi.MetadataResult{},
			expectedLen:   0,
			expectedIDs:   []string{},
			expectLabels:  []string{},
			expectNoError: true,
		},
		{
			name: "title with dot suffix",
			csvRows: []*ShortsCsvRow{
				createCSVRow("test1", "Ready in MB"),
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
			csvRows: []*ShortsCsvRow{
				createCSVRowWithStatus("test1", "Ready in MB", "Done"),
				createCSVRowWithStatus("test2", "Ready in MB", "In Progress"),
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
			csvRows: []*ShortsCsvRow{
				createCSVRowWithStatus("test1", "Ready in MB", ""),
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
			result := MapAndFilterShortsData(tt.csvRows, tt.mbItems)

			if tt.expectNoError {
				require.Len(t, result, tt.expectedLen, "unexpected result length")

				// Extract IDs and labels for comparison
				resultIDs := make([]string, 0, len(result))
				resultLabels := make([]string, 0, len(result))

				for _, item := range result {
					resultIDs = append(resultIDs, item.MBMetadata.ID)
					resultLabels = append(resultLabels, item.CSV.Label)
				}

				assert.ElementsMatch(t, tt.expectedIDs, resultIDs, "unexpected IDs in result")
				assert.ElementsMatch(t, tt.expectLabels, resultLabels, "unexpected labels in result")
			}
		})
	}
}
