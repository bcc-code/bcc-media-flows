package miscworkflows

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/fcplibrary"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

type CopyVXWithRelatedTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env    *testsuite.TestWorkflowEnvironment
	copies []activities.MoveFileInput
}

func TestCopyVXWithRelated(t *testing.T) {
	suite.Run(t, new(CopyVXWithRelatedTestSuite))
}

func (s *CopyVXWithRelatedTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.copies = nil
	s.env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).Maybe().Return(nil, nil)
}

func (s *CopyVXWithRelatedTestSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
}

func relation(source, target string) vsapi.Relation {
	return vsapi.Relation{Direction: vsapi.Direction{Source: source, Target: target}}
}

func (s *CopyVXWithRelatedTestSuite) expectRelations(vxID string, relations ...vsapi.Relation) {
	s.env.OnActivity(activities.Vidispine.GetRelations, mock.Anything, vxID).Once().Return(relations, nil)
}

func (s *CopyVXWithRelatedTestSuite) expectOriginal(vxID, path string) {
	s.env.OnActivity(activities.Vidispine.GetFileFromVXActivity, mock.Anything, vsactivity.GetFileFromVXParams{
		VXID: vxID,
		Tags: []vsapi.ShapeTag{vsapi.ShapeTagOriginal},
	}).Once().Return(&vsactivity.GetFileFromVXResult{
		FilePath: paths.New(paths.IsilonDrive, path),
		ShapeTag: vsapi.ShapeTagOriginal,
	}, nil)
}

func (s *CopyVXWithRelatedTestSuite) expectNoOriginal(vxID string) {
	s.env.OnActivity(activities.Vidispine.GetFileFromVXActivity, mock.Anything, vsactivity.GetFileFromVXParams{
		VXID: vxID,
		Tags: []vsapi.ShapeTag{vsapi.ShapeTagOriginal},
	}).Once().Return(nil, temporal.NewNonRetryableApplicationError("no shape found", "NoShape", nil))
}

func (s *CopyVXWithRelatedTestSuite) expectFolderAndCopies(n int) {
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Once().Return(nil, nil)
	if n == 0 {
		return
	}
	s.env.OnActivity(activities.Util.CopyFile, mock.Anything, mock.Anything).Times(n).Run(func(args mock.Arguments) {
		s.copies = append(s.copies, args.Get(1).(activities.MoveFileInput))
	}).Return(nil, nil)
}

func (s *CopyVXWithRelatedTestSuite) result() CopyVXWithRelatedResult {
	s.True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())
	var result CopyVXWithRelatedResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	return result
}

// copiedTo maps each copy's destination, relative to folder, to its source path.
func (s *CopyVXWithRelatedTestSuite) copiedTo(folder string) map[string]string {
	out := map[string]string{}
	for _, c := range s.copies {
		rel, err := filepath.Rel(folder, c.Destination.Linux())
		s.Require().NoError(err)
		out[rel] = c.Source.Path
	}
	return out
}

func (s *CopyVXWithRelatedTestSuite) Test_CopiesItemAndRelatedInEitherDirection() {
	s.expectRelations("VX-1", relation("VX-1", "VX-3"), relation("VX-2", "VX-1"))
	s.expectOriginal("VX-1", "Production/video/main.mxf")
	s.expectOriginal("VX-2", "Production/audio/main_nor.wav")
	s.expectOriginal("VX-3", "Production/audio/main_eng.wav")
	s.expectFolderAndCopies(3)

	s.env.ExecuteWorkflow(CopyVXWithRelated, CopyVXWithRelatedParams{VXID: "VX-1"})
	result := s.result()

	s.Len(result.Copied, 3)
	s.Empty(result.Skipped)
	s.Equal(map[string]string{
		"main.mxf":     "Production/video/main.mxf",
		"main_nor.wav": "Production/audio/main_nor.wav",
		"main_eng.wav": "Production/audio/main_eng.wav",
	}, s.copiedTo(result.Folder))
}

func (s *CopyVXWithRelatedTestSuite) Test_SkipsRelatedWithoutOriginalAndDuplicates() {
	s.expectRelations("VX-1", relation("VX-1", "VX-2"), relation("VX-2", "VX-1"), relation("VX-1", "VX-3"))
	s.expectOriginal("VX-1", "Production/video/main.mxf")
	s.expectOriginal("VX-2", "Production/audio/main_nor.wav")
	s.expectNoOriginal("VX-3")
	s.expectFolderAndCopies(2)

	s.env.ExecuteWorkflow(CopyVXWithRelated, CopyVXWithRelatedParams{VXID: "VX-1"})
	result := s.result()

	s.Len(result.Copied, 2)
	s.Equal([]string{"VX-3"}, result.Skipped)
}

func (s *CopyVXWithRelatedTestSuite) Test_PrefixesDuplicateFilenames() {
	s.expectRelations("VX-1", relation("VX-1", "VX-2"))
	s.expectOriginal("VX-1", "Production/a/file.mxf")
	s.expectOriginal("VX-2", "Production/b/file.mxf")
	s.expectFolderAndCopies(2)

	s.env.ExecuteWorkflow(CopyVXWithRelated, CopyVXWithRelatedParams{VXID: "VX-1"})
	result := s.result()

	s.Equal(map[string]string{
		"file.mxf":      "Production/a/file.mxf",
		"VX-2_file.mxf": "Production/b/file.mxf",
	}, s.copiedTo(result.Folder), "the main item keeps its name")
}

func (s *CopyVXWithRelatedTestSuite) Test_FailsWhenMainItemHasNoOriginal() {
	s.expectRelations("VX-1", relation("VX-1", "VX-2"))
	s.expectNoOriginal("VX-1")
	s.expectFolderAndCopies(0)

	s.env.ExecuteWorkflow(CopyVXWithRelated, CopyVXWithRelatedParams{VXID: "VX-1"})
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
	s.env.AssertActivityNotCalled(s.T(), "CopyFile", mock.Anything, mock.Anything)
}

func (s *CopyVXWithRelatedTestSuite) Test_RequiresVXID() {
	s.env.ExecuteWorkflow(CopyVXWithRelated, CopyVXWithRelatedParams{})
	s.True(s.env.IsWorkflowCompleted())
	s.ErrorContains(s.env.GetWorkflowError(), "VXID is required")
}

func (s *CopyVXWithRelatedTestSuite) Test_RestoresFCPLibraryWithItsMedia() {
	// VX-2 is in the library twice, VX-3 is in the library and related, VX-4 is only related.
	s.expectRelations("VX-1", relation("VX-1", "VX-3"), relation("VX-1", "VX-4"))
	s.expectOriginal("VX-1", "Production/projects/BIBX.fcpbundlezip")
	s.expectOriginal("VX-2", "Production/raw/clip.mxf")
	s.expectOriginal("VX-3", "Production/raw/song.wav")
	s.expectOriginal("VX-4", "Production/raw/extra.mov")
	s.expectNoOriginal("VX-5")
	s.expectFolderAndCopies(4)

	var library paths.Path
	s.env.OnActivity(activities.Util.ExtractFCPLibrary, mock.Anything, mock.Anything).Once().
		Return(func(_ context.Context, in activities.ExtractFCPLibraryInput) (*activities.ExtractFCPLibraryResult, error) {
			s.Equal("Production/projects/BIBX.fcpbundlezip", in.Archive.Path)
			library = in.Destination.Append("BIBX.fcpbundle")
			return &activities.ExtractFCPLibraryResult{
				Library: library,
				Items: []fcplibrary.Item{
					{VXID: "VX-2", Path: "Event/Original Media/clip.mxf"},
					{VXID: "VX-3", Path: "Event/Original Media/song.wav"},
					{VXID: "VX-2", Path: "OUT/Original Media/clip.mxf"},
					{VXID: "VX-5", Path: "Event/Original Media/missing.mov"},
				},
				Skipped: []fcplibrary.SkippedItem{{VXID: "VX-9", Path: "Users/x/a.plist", Reason: "outside"}},
			}, nil
		})

	s.env.ExecuteWorkflow(CopyVXWithRelated, CopyVXWithRelatedParams{VXID: "VX-1"})
	result := s.result()

	s.Equal(library.Linux(), result.Library)
	s.Equal(map[string]string{
		"BIBX.fcpbundle/Event/Original Media/clip.mxf": "Production/raw/clip.mxf",
		"BIBX.fcpbundle/Event/Original Media/song.wav": "Production/raw/song.wav",
		"BIBX.fcpbundle/OUT/Original Media/clip.mxf":   "Production/raw/clip.mxf",
		"extra.mov": "Production/raw/extra.mov",
	}, s.copiedTo(result.Folder), "the archive itself is not copied; related items outside the library go next to it")
	s.Equal([]string{"VX-5"}, result.Skipped)
	s.Len(result.LibrarySkipped, 1)
}
