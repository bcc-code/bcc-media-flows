package ingestworkflows

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/services/ingest"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type AddMetaTagsTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

type metaCall struct {
	Op    string
	Key   string
	Value string
}

func addMetaTagsWorkflow(ctx workflow.Context, metadata *ingest.Metadata) error {
	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())
	return addMetaTags(ctx, "VX-1", metadata)
}

func (s *AddMetaTagsTestSuite) run(metadata *ingest.Metadata) []metaCall {
	env := s.NewTestWorkflowEnvironment()

	var calls []metaCall
	record := func(op string) func(mock.Arguments) {
		return func(args mock.Arguments) {
			params := args.Get(1).(vsactivity.VXMetadataFieldParams)
			calls = append(calls, metaCall{Op: op, Key: params.Key, Value: params.Value})
		}
	}

	env.OnActivity(activities.Vidispine.SetVXMetadataFieldActivity, mock.Anything, mock.Anything).
		Run(record("set")).Return(nil, nil)
	env.OnActivity(activities.Vidispine.AddToVXMetadataFieldActivity, mock.Anything, mock.Anything).
		Run(record("add")).Return(nil, nil)

	env.ExecuteWorkflow(addMetaTagsWorkflow, metadata)
	require.True(s.T(), env.IsWorkflowCompleted())
	require.NoError(s.T(), env.GetWorkflowError())

	return calls
}

func fullMetadata() *ingest.Metadata {
	return &ingest.Metadata{
		JobProperty: ingest.JobProperty{
			SenderEmail:        "someone@bcc.media",
			JobID:              42,
			PersonsAppearing:   "Ada, Grace ,, Alan",
			Tags:               "news, sport",
			Language:           "nor",
			ProgramID:          "BTV - Brunstad TV",
			Season:             "3",
			Episode:            "7",
			EpisodeTitle:       "The Title",
			EpisodeDescription: "A description",
		},
	}
}

func (s *AddMetaTagsTestSuite) Test_CallOrderAndValues() {
	calls := s.run(fullMetadata())

	assert.Equal(s.T(), []metaCall{
		{Op: "set", Key: vscommon.FieldUploadedBy.Value, Value: "someone@bcc.media"},
		{Op: "set", Key: vscommon.FieldUploadJob.Value, Value: "42"},
		{Op: "add", Key: vscommon.FieldPersonsAppearing.Value, Value: "Ada"},
		{Op: "add", Key: vscommon.FieldPersonsAppearing.Value, Value: "Grace"},
		{Op: "add", Key: vscommon.FieldPersonsAppearing.Value, Value: "Alan"},
		{Op: "add", Key: vscommon.FieldGeneralTags.Value, Value: "news"},
		{Op: "add", Key: vscommon.FieldGeneralTags.Value, Value: "sport"},
		{Op: "set", Key: vscommon.FieldLanguagesRecorded.Value, Value: "nor"},
		{Op: "set", Key: vscommon.FieldProgram.Value, Value: "Brunstad TV"},
		{Op: "set", Key: vscommon.FieldSeason.Value, Value: "3"},
		{Op: "set", Key: vscommon.FieldEpisode.Value, Value: "7"},
		{Op: "set", Key: vscommon.FieldTitle.Value, Value: "Brunstad TV | The Title"},
		{Op: "set", Key: vscommon.FieldEpisodeDescription.Value, Value: "A description"},
	}, calls)
}

func (s *AddMetaTagsTestSuite) Test_EmptyFieldsAreSkipped() {
	metadata := &ingest.Metadata{
		JobProperty: ingest.JobProperty{SenderEmail: "a@b.c", JobID: 1},
	}

	calls := s.run(metadata)

	s.Len(calls, 2, "only the two unconditional calls")
}

func (s *AddMetaTagsTestSuite) Test_ProgramIDWithoutSeparatorIsTakenWhole() {
	metadata := fullMetadata()
	metadata.JobProperty.ProgramID = "Brunstad TV"

	calls := s.run(metadata)

	s.Contains(calls, metaCall{Op: "set", Key: vscommon.FieldProgram.Value, Value: "Brunstad TV"})
	s.Contains(calls, metaCall{Op: "set", Key: vscommon.FieldTitle.Value, Value: "Brunstad TV | The Title"})
}

func (s *AddMetaTagsTestSuite) Test_TitleIsNotPrefixedWithoutAProgram() {
	metadata := fullMetadata()
	metadata.JobProperty.ProgramID = ""

	calls := s.run(metadata)

	s.Contains(calls, metaCall{Op: "set", Key: vscommon.FieldTitle.Value, Value: "The Title"})
}

func TestAddMetaTagsTestSuite(t *testing.T) {
	suite.Run(t, new(AddMetaTagsTestSuite))
}
