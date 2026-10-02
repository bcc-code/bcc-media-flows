package miscworkflows

import (
	"errors"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type MoveStorageFilesTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *MoveStorageFilesTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *MoveStorageFilesTestSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
}

func TestMoveStorageFiles(t *testing.T) {
	suite.Run(t, new(MoveStorageFilesTestSuite))
}

var (
	storageFileA = vsactivity.StorageItemFile{FileID: "VX-F1", Path: "2023/a.mxf", Size: 100, ItemID: "VX-1", ShapeID: "VX-S1"}
	storageFileB = vsactivity.StorageItemFile{FileID: "VX-F2", Path: "2023/b.mxf", Size: 200, ItemID: "VX-2", ShapeID: "VX-S2"}
)

func storagePage(files ...vsactivity.StorageItemFile) *vsactivity.ListItemFilesOnStorageResult {
	return &vsactivity.ListItemFilesOnStorageResult{Files: len(files), Items: files}
}

func (s *MoveStorageFilesTestSuite) expectPage(offset int, page *vsactivity.ListItemFilesOnStorageResult) {
	s.env.OnActivity(vsactivity.Vidispine.ListItemFilesOnStorage, mock.Anything, vsactivity.ListItemFilesOnStorageParams{
		StorageID:  "VX-42",
		PathPrefix: "2023",
		Offset:     offset,
		Count:      moveStorageFilesPageSize,
	}).Once().Return(page, nil)
}

func (s *MoveStorageFilesTestSuite) expectMove(f vsactivity.StorageItemFile, err error) {
	s.env.OnActivity(activities.Cantemo.MoveFileWait, mock.Anything, &cantemo.RenameFileParams{
		ItemID:            f.ItemID,
		ShapeID:           f.ShapeID,
		SourceStorage:     "VX-42",
		DestinatinStorage: "VX-56",
		NewPath:           f.Path,
	}).Once().Return(nil, err)
}

func (s *MoveStorageFilesTestSuite) run(dryRun bool) *MoveStorageFilesResult {
	s.env.ExecuteWorkflow(MoveStorageFiles, MoveStorageFilesParams{
		SourceStorage:      "VX-42",
		DestinationStorage: "VX-56",
		PathPrefix:         "2023",
		DryRun:             dryRun,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var res MoveStorageFilesResult
	s.Require().NoError(s.env.GetWorkflowResult(&res))
	return &res
}

func (s *MoveStorageFilesTestSuite) Test_DryRunCountsWithoutMoving() {
	s.expectPage(0, storagePage(storageFileA, storageFileB))
	s.expectPage(2, storagePage())

	res := s.run(true)

	s.Equal(2, res.Files)
	s.EqualValues(300, res.Bytes)
	s.Equal([]string{"2023/a.mxf", "2023/b.mxf"}, res.SamplePaths)
	s.env.AssertActivityNotCalled(s.T(), "MoveFileWait", mock.Anything, mock.Anything)
}

func (s *MoveStorageFilesTestSuite) Test_MovesEveryFileAndRelistsFromStart() {
	s.expectPage(0, storagePage(storageFileA, storageFileB))
	s.expectMove(storageFileA, nil)
	s.expectMove(storageFileB, nil)
	s.expectPage(0, storagePage())

	res := s.run(false)

	s.Equal(2, res.Files)
	s.EqualValues(300, res.Bytes)
	s.Zero(res.Failed)
}

func (s *MoveStorageFilesTestSuite) Test_FailedMoveIsSkippedOnTheNextPage() {
	s.expectPage(0, storagePage(storageFileA, storageFileB))
	s.expectMove(storageFileA, errors.New("cantemo down"))
	s.expectMove(storageFileB, nil)
	// A is still on the source, ahead of everything else.
	s.expectPage(1, storagePage())

	res := s.run(false)

	s.Equal(1, res.Files)
	s.Equal(1, res.Failed)
	s.Equal([]string{"2023/a.mxf"}, res.FailedPaths)
}

func (s *MoveStorageFilesTestSuite) Test_FileStillOnSourceAfterMoveIsNotRetried() {
	s.expectPage(0, storagePage(storageFileA))
	s.expectMove(storageFileA, nil)
	// The move reported success, but the file is listed again.
	s.expectPage(0, storagePage(storageFileA))
	s.expectPage(1, storagePage())

	res := s.run(false)

	s.Equal(1, res.Files)
	s.Equal(1, res.Failed)
	s.Equal([]string{"2023/a.mxf"}, res.FailedPaths)
}

func (s *MoveStorageFilesTestSuite) Test_RejectsSameStorage() {
	s.env.ExecuteWorkflow(MoveStorageFiles, MoveStorageFilesParams{
		SourceStorage:      "VX-42",
		DestinationStorage: "VX-42",
	})
	s.Error(s.env.GetWorkflowError())
}

func (s *MoveStorageFilesTestSuite) Test_RejectsUnknownStorage() {
	s.env.ExecuteWorkflow(MoveStorageFiles, MoveStorageFilesParams{
		SourceStorage:      "VX-999",
		DestinationStorage: "VX-42",
	})
	s.Error(s.env.GetWorkflowError())
}
