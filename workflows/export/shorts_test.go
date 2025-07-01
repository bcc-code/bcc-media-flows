package export

import (
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"testing"
)

type testCase struct {
	Name      string
	Items     []vsapi.MetadataResult
	CSVRows   []ShortsCsvRow
	Expected  []ShortLanguageUpdate
	ExpectErr bool
}

type ShortsTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *ShortsTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func TestShortsSuite(t *testing.T) {
	suite.Run(t, new(ShortsTestSuite))
}

type MapAndFilterTestSuite struct {
	suite.Suite
}

func (s *MapAndFilterTestSuite) createMetadataResult(id, title string) *vsapi.MetadataResult {
	return &vsapi.MetadataResult{
		ID: id,
		Terse: map[string][]*vsapi.MetadataField{
			vscommon.FieldTitle.Value: {{
				Value: title,
			}},
		},
	}
}

func (s *MapAndFilterTestSuite) createCSVRow(label, editorialStatus string) *ShortsCsvRow {
	return &ShortsCsvRow{
		Label:           label,
		EditorialStatus: editorialStatus,
	}
}

func (s *MapAndFilterTestSuite) TestMapAndFilterShortsData() {
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
				s.createCSVRow("test1", "Ready in MB"),
				s.createCSVRow("test2", "Ready in MB"),
				s.createCSVRow("test3", "Not Ready"),
			},
			mbItems: []*vsapi.MetadataResult{
				s.createMetadataResult("id1", "test1"),
				s.createMetadataResult("id2", "test2"),
				s.createMetadataResult("id3", "test3"),
			},
			expectedLen:   2,
			expectedIDs:   []string{"id1", "id2"},
			expectLabels:  []string{"test1", "test2"},
			expectNoError: true,
		},
		{
			name: "no matches",
			csvRows: []*ShortsCsvRow{
				s.createCSVRow("test1", "Ready in MB"),
			},
			mbItems: []*vsapi.MetadataResult{
				s.createMetadataResult("id1", "different"),
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
				s.createCSVRow("test1", "Ready in MB"),
			},
			mbItems: []*vsapi.MetadataResult{
				s.createMetadataResult("id1", "test1.suffix"),
			},
			expectedLen:   1,
			expectedIDs:   []string{"id1"},
			expectLabels:  []string{"test1"},
			expectNoError: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := MapAndFilterShortsData(tt.csvRows, tt.mbItems)

			if tt.expectNoError {
				require.Len(s.T(), result, tt.expectedLen, "unexpected result length")

				// Extract IDs and labels for comparison
				resultIDs := make([]string, 0, len(result))
				resultLabels := make([]string, 0, len(result))

				for _, item := range result {
					resultIDs = append(resultIDs, item.MBMetadata.ID)
					resultLabels = append(resultLabels, item.CSV.Label)
				}

				assert.ElementsMatch(s.T(), tt.expectedIDs, resultIDs, "unexpected IDs in result")
				assert.ElementsMatch(s.T(), tt.expectLabels, resultLabels, "unexpected labels in result")
			}
		})
	}
}

func TestMapAndFilterSuite(t *testing.T) {
	suite.Run(t, new(MapAndFilterTestSuite))
}
