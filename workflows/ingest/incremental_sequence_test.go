package ingestworkflows

import (
	"context"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
)

// mainPath are the activities only doIncremental itself schedules. The preview
// coroutine runs alongside it, so anything it touches — TranscodeGrowingPreview,
// ImportFileAsShape, CloseFile — interleaves unpredictably and is left out.
var mainPath = []string{
	"CreatePlaceholderActivity",
	"AddFileToPlaceholder",
	"RsyncIncrementalCopy",
	"ListReaperFiles",
	"CreateThumbnailsActivity",
}

// TestIncrementalMainPathOrder pins the order the ingest does its work in.
// Extracting helpers does not change a workflow's history; reordering while
// moving code does, and that breaks replay for an ingest already running.
func TestIncrementalMainPathOrder(t *testing.T) {
	env := newIncrementalEnv(t)

	var calls []string
	env.OnActivity(activities.Audio.AnalyzeFile, mock.Anything, mock.Anything).
		Return(&ffmpeg.StreamInfo{TotalSeconds: incrementalTestDurationSeconds}, nil).Maybe()

	env.SetOnActivityStartedListener(func(info *activity.Info, _ context.Context, _ converter.EncodedValues) {
		calls = append(calls, info.ActivityType.Name)
	})

	runIncremental(t, env)

	got := lo.Filter(calls, func(name string, _ int) bool {
		return lo.Contains(mainPath, name)
	})

	assert.Equal(t, []string{
		"CreatePlaceholderActivity",
		"AddFileToPlaceholder",
		"RsyncIncrementalCopy",
		"RsyncIncrementalCopy",
		"ListReaperFiles",
		"CreateThumbnailsActivity",
	}, got)
}
