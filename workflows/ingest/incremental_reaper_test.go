package ingestworkflows

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
)

// listReaperFilesWorkflow drives the helper the way doIncremental does.
func listReaperFilesWorkflow(ctx workflow.Context, sessionID string) (*activities.ReaperResult, error) {
	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())
	return listReaperFiles(ctx, sessionID, "VX-1")
}

func runListReaperFiles(t *testing.T, sessionID string, setup func(env *testsuite.TestWorkflowEnvironment)) (*activities.ReaperResult, error) {
	t.Helper()

	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(listReaperFilesWorkflow)
	setup(env)

	env.ExecuteWorkflow(listReaperFilesWorkflow, sessionID)
	require.True(t, env.IsWorkflowCompleted())

	if err := env.GetWorkflowError(); err != nil {
		return nil, err
	}

	result := &activities.ReaperResult{}
	require.NoError(t, env.GetWorkflowResult(result))
	return result, nil
}

// A reaper that failed to start leaves no session, and asking for the files of a
// session that does not exist gets an error or another recording's files. The audio is
// skipped, and the video ingest carries on.
func TestListReaperFiles_SkipsTheListingWithoutASession(t *testing.T) {
	listed := false

	result, err := runListReaperFiles(t, "", func(env *testsuite.TestWorkflowEnvironment) {
		env.OnActivity(activities.Live.ListReaperFiles, mock.Anything, mock.Anything).Run(
			func(mock.Arguments) { listed = true }).Return(&activities.ReaperResult{}, nil).Maybe()
		env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).Return(
			&telegram.Message{}, nil).Maybe()
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Files, "no session means no audio to import")
	assert.False(t, listed, "the reaper must not be asked about a session that was never started")
}

// With a session, the listing is what decides which audio gets imported.
func TestListReaperFiles_ListsTheSession(t *testing.T) {
	var askedFor string

	result, err := runListReaperFiles(t, "session-1", func(env *testsuite.TestWorkflowEnvironment) {
		env.OnActivity(activities.Live.ListReaperFiles, mock.Anything, mock.Anything).Run(
			func(args mock.Arguments) {
				params := args.Get(1).(*activities.ListReaperFilesParams)
				askedFor = params.SessionID
			}).Return(&activities.ReaperResult{Files: []string{`C:\reaper\a.wav`}}, nil)
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "session-1", askedFor)
	assert.Len(t, result.Files, 1)
}

// A listing that fails is still a failure: the session exists, so the audio it recorded
// is expected, and quietly importing none of it would lose a live recording.
func TestListReaperFiles_ListingFailureIsAnError(t *testing.T) {
	_, err := runListReaperFiles(t, "session-1", func(env *testsuite.TestWorkflowEnvironment) {
		env.OnActivity(activities.Live.ListReaperFiles, mock.Anything, mock.Anything).
			Return(nil, assert.AnError)
	})

	require.Error(t, err)
}
