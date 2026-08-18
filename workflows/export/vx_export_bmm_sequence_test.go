package export

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	platform_activities "github.com/bcc-code/bcc-media-flows/activities/platform"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-platform/backend/asset"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

// BMMSequenceTestSuite records the order VXExportToBMM schedules its activities in.
// Extracting helpers does not change a workflow's history, but reordering while
// moving code does, and that breaks replay for anything in flight.
type BMMSequenceTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func (s *BMMSequenceTestSuite) Test_ActivityOrder() {
	env := s.NewTestWorkflowEnvironment()

	var calls []string
	record := func(name string) func(mock.Arguments) {
		return func(mock.Arguments) { calls = append(calls, name) }
	}

	env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).Maybe().Return(nil, nil)
	env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).
		Run(record("CreateFolder")).Return(nil, nil)
	env.OnActivity(activities.Audio.NormalizeAudioActivity, mock.Anything, mock.Anything).
		Run(record("Normalize")).Return(&activities.NormalizeAudioResult{
		FilePath: paths.MustParse("/mnt/temp/normalized.wav"),
	}, nil)
	env.OnActivity(activities.Audio.TranscodeToAudioAac, mock.Anything, mock.Anything).
		Run(record("Aac")).Return(&common.AudioResult{
		OutputPath: paths.MustParse("/mnt/temp/out.aac"),
		Format:     "aac",
	}, nil)
	env.OnActivity(activities.Audio.TranscodeToAudioMP3, mock.Anything, mock.Anything).
		Run(record("Mp3")).Return(&common.AudioResult{
		OutputPath: paths.MustParse("/mnt/temp/out.mp3"),
		Format:     "mp3",
	}, nil)
	env.OnActivity(activities.Util.MoveFile, mock.Anything, mock.Anything).
		Run(record("MoveTranscript")).Return(nil, nil)
	env.OnActivity(activities.Util.RcloneWaitForJob, mock.Anything, mock.Anything).Maybe().Return(true, nil)
	env.OnActivity(activities.Platform.GetTimedMetadataChaptersActivity, mock.Anything, mock.Anything).
		Run(record("Chapters")).Return([]asset.TimedMetadata{}, nil)
	env.OnActivity(activities.Util.WriteFile, mock.Anything, mock.Anything).
		Run(record("WriteJSON")).Return(nil, nil)
	env.OnActivity(activities.Util.RcloneCopyDir, mock.Anything, mock.Anything).
		Run(record("CopyDir")).Return(1, nil)
	env.OnActivity(activities.Util.TriggerBMMImport, mock.Anything, mock.Anything).
		Run(record("TriggerImport")).Return(true, nil)

	params := VXExportChildWorkflowParams{
		ParentParams:      VXExportParams{VXID: "VX-1"},
		ExportDestination: AssetExportDestinationBMM,
		TempDir:           paths.MustParse("/mnt/temp/workflows"),
		OutputDir:         paths.MustParse("/mnt/temp/workflows/output"),
		MergeResult: MergeExportDataResult{
			AudioFiles:     map[string]paths.Path{"nor": paths.MustParse("/mnt/temp/nor.wav")},
			JSONTranscript: map[string]paths.Path{"no": paths.MustParse("/mnt/temp/no.json")},
		},
	}

	env.ExecuteWorkflow(VXExportToBMM, params)
	require.True(s.T(), env.IsWorkflowCompleted())
	require.NoError(s.T(), env.GetWorkflowError())

	encodes := len(aacBitrates) + len(mp3Bitrates)
	require.Len(s.T(), calls, 2+encodes+5)

	// The encodes are scheduled together and run in whatever order the worker gets
	// to them, so only their phase and their number are fixed.
	assert.Equal(s.T(), []string{"CreateFolder", "Normalize"}, calls[:2],
		"the output folder exists before anything writes into it")

	batch := calls[2 : 2+encodes]
	assert.Equal(s.T(), len(aacBitrates), lo.Count(batch, "Aac"))
	assert.Equal(s.T(), len(mp3Bitrates), lo.Count(batch, "Mp3"))

	assert.Equal(s.T(), []string{
		"MoveTranscript", "Chapters", "WriteJSON", "CopyDir", "TriggerImport",
	}, calls[2+encodes:], "everything after the encodes is strictly sequential")
}

func TestBMMSequence(t *testing.T) {
	suite.Run(t, new(BMMSequenceTestSuite))
}

var _ = platform_activities.GetTimedMetadataChaptersParams{}
