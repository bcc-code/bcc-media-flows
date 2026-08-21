package export

import (
	"context"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
)

// TestGenerateShortActivityOrder pins the order GenerateShort works in.
// Extracting helpers does not change a workflow's history; reordering while
// moving code does.
func TestGenerateShortActivityOrder(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	videoFile := paths.MustParse("/mnt/temp/workflows/clip.mxf")

	env.OnActivity(activities.Vidispine.GetExportDataActivity, mock.Anything, mock.Anything).
		Return(&vidispine.ExportData{
			Title: "A Title",
			Clips: []*vidispine.Clip{{VideoFile: videoFile.Local()}},
		}, nil)
	env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Return("", nil).Maybe()
	env.OnWorkflow(MergeExportData, mock.Anything, mock.Anything).
		Return(&MergeExportDataResult{
			VideoFile:  &videoFile,
			AudioFiles: map[string]paths.Path{"nor": paths.MustParse("/mnt/temp/workflows/nor.wav")},
		}, nil)
	env.OnActivity(activities.Video.FFmpegGetSceneChanges, mock.Anything, mock.Anything).
		Return([]float64{1.0}, nil)
	env.OnActivity(activities.Util.SubmitShortJobActivity, mock.Anything, mock.Anything).
		Return(&activities.SubmitShortJobResult{JobID: "job-1"}, nil)
	env.OnActivity(activities.Util.CheckJobStatusActivity, mock.Anything, mock.Anything).
		Return(&activities.GenerateShortRequestResult{
			Status:    "completed",
			Keyframes: []activities.Keyframe{},
		}, nil)
	env.OnActivity(activities.Video.CropShortActivity, mock.Anything, mock.Anything).
		Return(&activities.CropShortResult{Arguments: []string{"-i", "in.mxf"}}, nil)
	env.OnWorkflow(miscworkflows.ExecuteFFmpeg, mock.Anything, mock.Anything).Return(nil)

	var calls []string
	env.SetOnActivityStartedListener(func(info *activity.Info, _ context.Context, _ converter.EncodedValues) {
		calls = append(calls, info.ActivityType.Name)
	})

	env.ExecuteWorkflow(GenerateShort, GenerateShortDataParams{
		VXID:          "VX-1",
		OutputDirPath: "/mnt/isilon/Export/out",
		InSeconds:     0,
		OutSeconds:    10,
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	assert.Equal(t, []string{
		"GetExportDataActivity",
		"CreateFolder",
		"CreateFolder",
		"FFmpegGetSceneChanges",
		"SubmitShortJobActivity",
		"CheckJobStatusActivity",
		"CropShortActivity",
	}, calls)

	var _ = vsactivity.GetExportDataParams{}
}
