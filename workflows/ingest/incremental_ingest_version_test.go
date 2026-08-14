package ingestworkflows

import (
	"testing"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// The source and the preview report the same duration, so the catch-up wait
// finishes on its first check when it runs at all.
const incrementalTestDurationSeconds = 120.0

// newIncrementalEnv mocks everything doIncremental reaches, so a test can say
// something about one activity without the rest getting in the way. Only
// AnalyzeFile is left un-mocked here — each test sets it up itself, because
// whether it is called at all is the thing under test.
func newIncrementalEnv(t *testing.T) *testsuite.TestWorkflowEnvironment {
	t.Helper()

	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	env.OnActivity(activities.Vidispine.CreatePlaceholderActivity, mock.Anything, mock.Anything).
		Return(&vsactivity.CreatePlaceholderResult{AssetID: "VX-1"}, nil).Maybe()
	env.OnActivity(activities.Vidispine.SetVXMetadataFieldActivity, mock.Anything, mock.Anything).
		Return(nil, nil).Maybe()
	env.OnActivity(activities.Vidispine.AddFileToPlaceholder, mock.Anything, mock.Anything).
		Return(&vsactivity.FileJobResult{FileID: "FILE-1"}, nil).Maybe()
	env.OnActivity(activities.Vidispine.ImportFileAsShapeActivity, mock.Anything, mock.Anything).
		Return(&vsactivity.ImportFileResult{JobID: "job-1"}, nil).Maybe()
	env.OnActivity(activities.Vidispine.CloseFile, mock.Anything, mock.Anything).
		Return(nil, nil).Maybe()
	env.OnActivity(activities.Vidispine.CreateThumbnailsActivity, mock.Anything, mock.Anything).
		Return(nil, nil).Maybe()
	env.OnActivity(activities.Vidispine.GetRelatedAudioFiles, mock.Anything, mock.Anything).
		Return(map[string]string{}, nil).Maybe()
	env.OnActivity(activities.Vidispine.JobCompleteOrErr, mock.Anything, mock.Anything).
		Return(true, nil).Maybe()

	env.OnActivity(activities.Live.StartReaper, mock.Anything, mock.Anything).
		Return("session-1", nil).Maybe()
	env.OnActivity(activities.Live.ListReaperFiles, mock.Anything, mock.Anything).
		Return(&activities.ReaperResult{}, nil).Maybe()
	env.OnActivity(activities.Live.RsyncIncrementalCopy, mock.Anything, mock.Anything).
		Return(&activities.RsyncIncrementalCopyResult{Size: 1024}, nil).Maybe()

	env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).
		Return("", nil).Maybe()
	env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).
		Return(nil, nil).Maybe()
	env.OnActivity(activities.Util.PokeFileCatalyst, mock.Anything, mock.Anything).
		Return(true, nil).Maybe()

	env.OnActivity(activities.Video.TranscodeGrowingPreview, mock.Anything, mock.Anything).
		Return(nil, nil).Maybe()

	env.OnWorkflow(miscworkflows.TranscribeVX, mock.Anything, mock.Anything).Return(nil).Maybe()
	env.OnWorkflow(miscworkflows.FixDurationVX, mock.Anything, mock.Anything).Return(nil).Maybe()
	env.OnWorkflow(ImportAudioFileFromReaper, mock.Anything, mock.Anything).Return(nil).Maybe()
	env.OnWorkflow(IngestSyncFix, mock.Anything, mock.Anything).Return(nil).Maybe()

	// Ends the copy loop on the first pass.
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(FileTransferredSignal, "/mnt/filecatalyst/ingestgrow/TEST_MU1.mxf")
	}, time.Second)

	return env
}

func runIncremental(t *testing.T, env *testsuite.TestWorkflowEnvironment) {
	t.Helper()

	env.ExecuteWorkflow(Incremental, IncrementalParams{
		Path:            "/mnt/filecatalyst/ingestgrow/TEST_MU1.mxf",
		ReaperSessionID: "session-1",
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

// An execution started before this change has a history without the catch-up
// wait's activities and timers. Replaying it must not produce them, or the
// workflow task fails and Temporal retries it until someone intervenes.
func TestIncrementalPreviewCatchUpVersion_OldExecutionsSkipIt(t *testing.T) {
	env := newIncrementalEnv(t)

	env.OnGetVersion(versionPreviewCatchUp, workflow.DefaultVersion, 1).
		Return(workflow.DefaultVersion)

	analyzed := 0
	env.OnActivity(activities.Audio.AnalyzeFile, mock.Anything, mock.Anything).
		Return(&ffmpeg.StreamInfo{TotalSeconds: incrementalTestDurationSeconds}, nil).
		Run(func(mock.Arguments) { analyzed++ }).Maybe()

	runIncremental(t, env)

	require.Zero(t, analyzed, "the catch-up wait must not run on a pre-change execution")
}

// New executions get the wait, which is the whole point of it.
func TestIncrementalPreviewCatchUpVersion_NewExecutionsWait(t *testing.T) {
	env := newIncrementalEnv(t)

	analyzed := 0
	env.OnActivity(activities.Audio.AnalyzeFile, mock.Anything, mock.Anything).
		Return(&ffmpeg.StreamInfo{TotalSeconds: incrementalTestDurationSeconds}, nil).
		Run(func(mock.Arguments) { analyzed++ })

	runIncremental(t, env)

	// The source, then one check of the preview, which already matches.
	require.Equal(t, 2, analyzed)
}
