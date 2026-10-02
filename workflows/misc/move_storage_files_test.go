package miscworkflows

import (
	"errors"
	"testing"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/temporal"
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
	// Already two folders deep, so it keeps its path.
	storageFileA = vsactivity.StorageItemFile{FileID: "VX-F1", Path: "2023/05/a.mxf", Size: 100, ItemID: "VX-1", ShapeID: "VX-S1"}
	// One folder deep, so it goes under the item's creation month.
	storageFileB     = vsactivity.StorageItemFile{FileID: "VX-F2", Path: "misc/b.mxf", Size: 200, ItemID: "VX-2", ShapeID: "VX-S2"}
	storageFileBDest = "2021/11/misc/b.mxf"
	itemBCreated     = time.Date(2021, 11, 3, 9, 0, 0, 0, time.UTC)
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

func (s *MoveStorageFilesTestSuite) expectCreated(itemID string, created time.Time, err error) {
	s.env.OnActivity(activities.Cantemo.GetItemCreated, mock.Anything, cantemo.GetItemCreatedParams{ItemID: itemID}).
		Once().Return(created, err)
}

func (s *MoveStorageFilesTestSuite) expectMove(f vsactivity.StorageItemFile, newPath string, err error) {
	s.env.OnActivity(activities.Cantemo.MoveFileWait, mock.Anything, &cantemo.RenameFileParams{
		ItemID:            f.ItemID,
		ShapeID:           f.ShapeID,
		SourceStorage:     "VX-42",
		DestinatinStorage: "VX-56",
		NewPath:           newPath,
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
	s.expectCreated("VX-2", itemBCreated, nil)
	s.expectPage(2, storagePage())

	res := s.run(true)

	s.Equal(2, res.Files)
	s.EqualValues(300, res.Bytes)
	s.Equal(1, res.Redated)
	s.Equal([]string{"2023/05/a.mxf -> 2023/05/a.mxf", "misc/b.mxf -> " + storageFileBDest}, res.SamplePaths)
	s.env.AssertActivityNotCalled(s.T(), "MoveFileWait", mock.Anything, mock.Anything)
}

func (s *MoveStorageFilesTestSuite) Test_MovesEveryFileAndRelistsFromStart() {
	s.expectPage(0, storagePage(storageFileA, storageFileB))
	s.expectMove(storageFileA, storageFileA.Path, nil)
	s.expectCreated("VX-2", itemBCreated, nil)
	s.expectMove(storageFileB, storageFileBDest, nil)
	s.expectPage(0, storagePage())

	res := s.run(false)

	s.Equal(2, res.Files)
	s.EqualValues(300, res.Bytes)
	s.Equal(1, res.Redated)
	s.Zero(res.Failed)
}

func (s *MoveStorageFilesTestSuite) Test_FailedMoveIsSkippedOnTheNextPage() {
	s.expectPage(0, storagePage(storageFileA, storageFileB))
	s.expectMove(storageFileA, storageFileA.Path, errors.New("cantemo down"))
	s.expectCreated("VX-2", itemBCreated, nil)
	s.expectMove(storageFileB, storageFileBDest, nil)
	// A is still on the source, ahead of everything else.
	s.expectPage(1, storagePage())

	res := s.run(false)

	s.Equal(1, res.Files)
	s.Equal(1, res.Failed)
	s.Equal([]string{"2023/05/a.mxf"}, res.FailedPaths)
}

func (s *MoveStorageFilesTestSuite) Test_FileStillOnSourceAfterMoveIsNotRetried() {
	s.expectPage(0, storagePage(storageFileA))
	s.expectMove(storageFileA, storageFileA.Path, nil)
	// The move reported success, but the file is listed again.
	s.expectPage(0, storagePage(storageFileA))
	s.expectPage(1, storagePage())

	res := s.run(false)

	s.Equal(1, res.Files)
	s.Equal(1, res.Failed)
	s.Equal([]string{"2023/05/a.mxf"}, res.FailedPaths)
}

func (s *MoveStorageFilesTestSuite) Test_FileAtRootGoesUnderCreationMonth() {
	root := vsactivity.StorageItemFile{FileID: "VX-F3", Path: "c.mxf", Size: 1, ItemID: "VX-3", ShapeID: "VX-S3"}
	s.expectPage(0, storagePage(root))
	s.expectCreated("VX-3", time.Date(2019, 2, 28, 0, 0, 0, 0, time.UTC), nil)
	s.expectMove(root, "2019/02/c.mxf", nil)
	s.expectPage(0, storagePage())

	res := s.run(false)

	s.Equal(1, res.Redated)
}

func (s *MoveStorageFilesTestSuite) Test_ShallowFileWithoutCreationDateIsNotMoved() {
	s.expectPage(0, storagePage(storageFileB))
	s.expectCreated("VX-2", time.Time{}, temporal.NewNonRetryableApplicationError("no date", "NO_CREATION_DATE", nil))
	s.expectPage(1, storagePage())

	res := s.run(false)

	s.Zero(res.Files)
	s.Equal(1, res.Failed)
	s.Equal([]string{"misc/b.mxf"}, res.FailedPaths)
	s.env.AssertActivityNotCalled(s.T(), "MoveFileWait", mock.Anything, mock.Anything)
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
