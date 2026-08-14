package ingestworkflows

import (
	"context"
	"testing"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

var (
	catchUpSourcePath  = paths.MustParse("/mnt/isilon/system/raw/TEST_MU1.mxf")
	catchUpPreviewPath = paths.MustParse("/mnt/isilon/system/preview/TEST_MU1.mp4")
)

const catchUpSourceSeconds = 120.0

// waitForPreviewCatchUpWorkflow reports how long the wait took on the workflow
// clock. The activity options are the ones doIncremental runs with, so the
// probe's own options are exercised the way they are in the ingest.
func waitForPreviewCatchUpWorkflow(ctx workflow.Context) (time.Duration, error) {
	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	start := workflow.Now(ctx)
	waitForPreviewToCatchUp(ctx, catchUpSourcePath, catchUpPreviewPath)
	return workflow.Now(ctx).Sub(start), nil
}

// runPreviewCatchUp plays previewSamples back to the wait, one per probe of the
// preview, holding the last value once they run out. It returns how long the
// wait took and how many times the preview was probed.
func runPreviewCatchUp(t *testing.T, previewSamples []float64) (time.Duration, int) {
	t.Helper()

	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(waitForPreviewCatchUpWorkflow)

	probes := 0
	env.OnActivity(activities.Audio.AnalyzeFile, mock.Anything, mock.Anything).Return(
		func(_ context.Context, input activities.AnalyzeFileParams) (*ffmpeg.StreamInfo, error) {
			if input.FilePath == catchUpSourcePath {
				return &ffmpeg.StreamInfo{TotalSeconds: catchUpSourceSeconds}, nil
			}

			seconds := previewSamples[min(probes, len(previewSamples)-1)]
			probes++
			return &ffmpeg.StreamInfo{TotalSeconds: seconds}, nil
		})

	env.ExecuteWorkflow(waitForPreviewCatchUpWorkflow)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var waited time.Duration
	require.NoError(t, env.GetWorkflowResult(&waited))
	return waited, probes
}

// The preview remux is synchronous and lengthens as the recording grows, so a
// probe landing inside one reads the duration it read last time while ffmpeg is
// still working. Cancelling on that truncates the preview, which is what this
// wait exists to prevent.
func TestPreviewCatchUpToleratesAStaleSample(t *testing.T) {
	waited, probes := runPreviewCatchUp(t, []float64{30, 30, 90, catchUpSourceSeconds})

	assert.Equal(t, 4, probes, "one repeated duration must not end the wait")
	assert.Less(t, waited, previewCatchUpDeadline)
}

func TestPreviewCatchUpStopsOnceTheTranscodeIsReallyStalled(t *testing.T) {
	waited, probes := runPreviewCatchUp(t, []float64{30})

	// The first probe sets the baseline, then previewCatchUpStaleSamples of them
	// report nothing new.
	assert.Equal(t, 1+previewCatchUpStaleSamples, probes)
	assert.Less(t, waited, previewCatchUpDeadline, "a stall should not wait out the deadline")
}

// A preview that keeps growing but never reaches the source must still let the
// ingest go, and within the deadline rather than a count of iterations.
func TestPreviewCatchUpGivesUpAtTheDeadline(t *testing.T) {
	crawling := make([]float64, 0, 60)
	for i := 0; i < 60; i++ {
		crawling = append(crawling, float64(i))
	}

	waited, probes := runPreviewCatchUp(t, crawling)

	assert.Greater(t, probes, 1)
	assert.GreaterOrEqual(t, waited, previewCatchUpDeadline)
	// The final sleep can carry it one interval past the deadline, no further.
	assert.Less(t, waited, previewCatchUpDeadline+2*previewCatchUpInterval)
}

// A preview that cannot be measured at all must let the ingest go rather than
// fail it, and must do so within the deadline.
//
// This does not cover the probe's retry policy: the test environment does not
// charge retry backoff to the workflow clock, so a probe that retries ten times
// looks the same here as one that does not. Against a real server the backoff is
// wall-clock time that the deadline counts, which is why the probe is
// configured for a single attempt.
func TestPreviewCatchUpIsBoundedWhenTheProbeKeepsFailing(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(waitForPreviewCatchUpWorkflow)

	env.OnActivity(activities.Audio.AnalyzeFile, mock.Anything, mock.Anything).Return(
		func(_ context.Context, input activities.AnalyzeFileParams) (*ffmpeg.StreamInfo, error) {
			if input.FilePath == catchUpSourcePath {
				return &ffmpeg.StreamInfo{TotalSeconds: catchUpSourceSeconds}, nil
			}
			return nil, assert.AnError
		})

	env.ExecuteWorkflow(waitForPreviewCatchUpWorkflow)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError(), "an unmeasurable preview must not fail the ingest")

	var waited time.Duration
	require.NoError(t, env.GetWorkflowResult(&waited))
	assert.Less(t, waited, previewCatchUpDeadline+2*previewCatchUpInterval)
}

// The wait is over as soon as the preview covers the source, within tolerance.
func TestPreviewCatchUpReturnsWhenThePreviewCoversTheSource(t *testing.T) {
	waited, probes := runPreviewCatchUp(t, []float64{catchUpSourceSeconds - previewCatchUpTolerance})

	assert.Equal(t, 1, probes)
	assert.Equal(t, previewCatchUpInterval, waited)
}
