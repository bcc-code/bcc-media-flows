package miscworkflows

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type TranscribeVXTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

// The transcriber can't write to Isilon, so it gets the temp folder, and the
// transcripts are copied to aux before anything imports them
func (s *TranscribeVXTestSuite) Test_TranscribesToTempAndImportsFromAux() {
	env := s.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(ImportSidecarSubtitle)

	audio := paths.MustParse("/mnt/temp/workflows/run/audio.wav")

	env.OnActivity(activities.Vidispine.GetFileFromVXActivity, mock.Anything, mock.Anything).
		Return(&vsactivity.GetFileFromVXResult{FilePath: paths.MustParse("/mnt/isilon/lowres.mp4")}, nil)
	env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Return(nil, nil)
	env.OnActivity(activities.Audio.PrepareForTranscription, mock.Anything, mock.Anything).
		Return(&activities.PrepareTranscriptionResult{OutputPath: &audio, HasAudio: true}, nil)

	var transcribeInput activities.TranscribeParams
	env.OnActivity(activities.Transcriber.Transcribe, mock.Anything, mock.MatchedBy(func(input activities.TranscribeParams) bool {
		transcribeInput = input
		return true
	})).Return(func(_ context.Context, input activities.TranscribeParams) (*activities.TranscribeResponse, error) {
		out := input.DestinationPath
		return &activities.TranscribeResponse{
			JSONPath:     out.Append("audio.json"),
			SRTPath:      out.Append("audio.srt"),
			WordsSRTPath: out.Append("audio.words.srt"),
			TXTPath:      out.Append("audio.txt"),
		}, nil
	})

	var mu sync.Mutex
	var copies []activities.MoveFileInput
	env.OnActivity(activities.Util.CopyFile, mock.Anything, mock.Anything).
		Return(func(_ context.Context, input activities.MoveFileInput) (*activities.FileResult, error) {
			mu.Lock()
			defer mu.Unlock()
			copies = append(copies, input)
			return &activities.FileResult{Path: input.Destination}, nil
		})

	var imported []paths.Path
	env.OnActivity(activities.Vidispine.ImportFileAsShapeActivity, mock.Anything, mock.Anything).
		Return(func(_ context.Context, input vsactivity.ImportFileAsShapeParams) (*vsactivity.ImportFileResult, error) {
			mu.Lock()
			defer mu.Unlock()
			imported = append(imported, input.FilePath)
			return &vsactivity.ImportFileResult{JobID: "VX-JOB-1"}, nil
		})
	env.OnActivity(activities.Vidispine.JobCompleteOrErr, mock.Anything, mock.Anything).Return(true, nil)

	var sidecar vsactivity.ImportSubtitleAsSidecarParams
	env.OnActivity(activities.Vidispine.ImportFileAsSidecarActivity, mock.Anything, mock.MatchedBy(
		func(input vsactivity.ImportSubtitleAsSidecarParams) bool {
			sidecar = input
			return true
		},
	)).Return(&vsactivity.ImportFileAsSidecarResult{}, nil)

	var readPath paths.Path
	env.OnActivity(activities.Util.ReadFile, mock.Anything, mock.MatchedBy(func(input activities.FileInput) bool {
		readPath = input.Path
		return true
	})).Return([]byte("transcript"), nil)
	env.OnActivity(activities.Vidispine.SetVXMetadataFieldActivity, mock.Anything, mock.Anything).
		Return(&vsactivity.SetVXMetadataFieldResult{}, nil)

	env.ExecuteWorkflow(TranscribeVX, TranscribeVXInput{Language: "no", VXID: "VX-1"})

	s.Require().True(env.IsWorkflowCompleted())
	s.Require().NoError(env.GetWorkflowError())

	s.Equal(paths.TempDrive, transcribeInput.DestinationPath.Drive)

	isAux := func(p paths.Path) bool {
		return p.Drive == paths.IsilonDrive && strings.HasPrefix(p.Path, "Production/aux/")
	}

	s.Require().Len(copies, 3)
	for _, c := range copies {
		s.Equal(paths.TempDrive, c.Source.Drive, "copied from %s", c.Source.Path)
		s.True(isAux(c.Destination), "copied to %s", c.Destination.Path)
	}

	s.Require().Len(imported, 2)
	for _, p := range imported {
		s.True(isAux(p), "imported %s", p.Path)
	}
	s.True(isAux(sidecar.FilePath), "sidecar %s", sidecar.FilePath.Path)
	s.True(isAux(readPath), "read %s", readPath.Path)
	s.Equal("audio.txt", readPath.Base())
}

func TestTranscribeVXTestSuite(t *testing.T) {
	suite.Run(t, new(TranscribeVXTestSuite))
}
