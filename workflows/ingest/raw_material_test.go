package ingestworkflows

import (
	"encoding/json"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type RawMaterialTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *RawMaterialTestSuite) SetupTest() {
	// Disable some timeout detection for easier debugging
	// This only works if set outside of the program!
	// os.Setenv("TEMPORAL_DEBUG", "true")

	s.env = s.NewTestWorkflowEnvironment()
}

func (s *RawMaterialTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

const rawMaterialFormJSON = `{"OrderForm":{"Value":"Rawmaterial"},"Targets":["test@example.com"],"Metadata":{"XMLName":{"Space":"","Local":"Metadata"},"JobProperty":{"JobID":6006,"UserName":"user.name","CompanyName":"","SourceIP":"1.2.3.4","UserEmail":"test@example.com","IngestStation":"filecatalyst-01/10.12.135.2","UploadBitRate":"39008 kbps","UploadTime":"00:32:42","FtpSiteID":"siteid","FileCount":4,"OrderForm":"Rawmaterial","SubmissionDate":"Wed Jun 05 15:39:38 CEST 2024","LastDateChanged":"Wed Jun 05 15:39:38 CEST 2024","Status":"pending","AssetType":"RAW","SenderEmail":"test@example.com","EpisodeTitle":"","EpisodeDescription":"","ProgramPost":"","ProgramID":"","Season":"","Episode":"","ReceivedFilename":"","PersonsAppearing":"","Tags":"","PromoType":"","Language":""},"FileList":{"Files":[{"FileName":"","IsFolder":false,"FileSize":29126656,"FilePath":"/files/6006"},{"FileName":"","IsFolder":false,"FileSize":25198336,"FilePath":"/files/6006"},{"FileName":"","IsFolder":false,"FileSize":9559295844,"FilePath":"/files/6006"},{"FileName":"","IsFolder":false,"FileSize":183379456,"FilePath":"/files/6006"}]},"JobHistoryLog":{"JobLogs":[{"LogID":0,"JobLogDate":"Wed Jun 05 15:39:38 CEST 2024","JobLogDescription":"status changed to 'submitted'","JobLogBy":""}]}},"Directory":{"Drive":"temp","Path":"workflows/314a5b8d-3f49-4797-bc56-8d32f11aefca/fc"}}`

func (s *RawMaterialTestSuite) Test_RawMaterialForm_InvalidFilename() {
	form := RawMaterialFormParams{}
	err := json.Unmarshal([]byte(rawMaterialFormJSON), &form)
	s.NoError(err)

	s.env.OnActivity(activities.Util.ListFiles, mock.Anything, mock.Anything).Return(
		paths.Files{
			//paths.MustParse("./testdata/v i d e o.mxf"),
			paths.MustParse("./testdata/video.mp4"),
			paths.MustParse("./testdata/æøå.mp4"),
		}, nil)

	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).
		Once().
		Return("", nil)

	s.env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).
		Once().
		Return(nil, nil)

	s.env.OnActivity(activities.Util.SendEmail, mock.Anything, mock.Anything).
		Once().
		Return(nil, nil)

	s.env.ExecuteWorkflow(RawMaterialForm, form)
	s.True(s.env.IsWorkflowCompleted())

	err = s.env.GetWorkflowError()
	s.Equal(err.Error(), `workflow execution error (type: RawMaterialForm, workflowID: default-test-workflow-id, runID: default-test-run-id): invalid filename: {{test} video.mp4}`)
}

// Only files with a video stream go through QC, and the whole upload is handed
// to one QC workflow so the uploader gets a single mail.
func (s *RawMaterialTestSuite) Test_RawMaterial_StartsQCForVideoFilesOnly() {
	video := paths.New(paths.IsilonDrive, "Input/Rawmaterial/CLIP_01.mxf")
	audio := paths.New(paths.IsilonDrive, "Input/Rawmaterial/AUDIO_01.wav")
	params := RawMaterialParams{
		FilesToIngest: paths.Files{video, audio},
		Language:      "no",
		Recipients:    []string{"uploader@example.com"},
	}

	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Once().Return(nil, nil)
	s.env.OnActivity(activities.Util.MoveFile, mock.Anything, mock.Anything).Times(2).Return(nil, nil)

	s.env.OnActivity(activities.Vidispine.CreatePlaceholderActivity, mock.Anything, vsactivity.CreatePlaceholderParams{Title: "CLIP_01.mxf"}).
		Once().Return(&vsactivity.CreatePlaceholderResult{AssetID: "VX-VIDEO"}, nil)
	s.env.OnActivity(activities.Vidispine.CreatePlaceholderActivity, mock.Anything, vsactivity.CreatePlaceholderParams{Title: "AUDIO_01.wav"}).
		Once().Return(&vsactivity.CreatePlaceholderResult{AssetID: "VX-AUDIO"}, nil)
	s.env.OnActivity(activities.Vidispine.ImportFileAsShapeActivity, mock.Anything, mock.Anything).Times(2).Return(nil, nil)
	s.env.OnActivity(activities.Vidispine.JobCompleteOrErr, mock.Anything, mock.Anything).Times(2).Return(true, nil)

	s.env.OnActivity(activities.Audio.AnalyzeFile, mock.Anything, mock.MatchedBy(func(in activities.AnalyzeFileParams) bool {
		return in.FilePath.Base() == "CLIP_01.mxf"
	})).Once().Return(&ffmpeg.StreamInfo{HasVideo: true, HasAudio: true}, nil)
	s.env.OnActivity(activities.Audio.AnalyzeFile, mock.Anything, mock.MatchedBy(func(in activities.AnalyzeFileParams) bool {
		return in.FilePath.Base() == "AUDIO_01.wav"
	})).Once().Return(&ffmpeg.StreamInfo{HasAudio: true}, nil)

	s.env.OnActivity(activities.Vidispine.CreateThumbnailsActivity, mock.Anything, vsactivity.CreateThumbnailsParams{AssetID: "VX-VIDEO"}).
		Once().Return(nil, nil)

	// The whole upload is read through ffmpeg in one go before anything is imported.
	var demuxInput miscworkflows.DemuxCheckInput
	s.env.OnWorkflow(miscworkflows.DemuxCheck, mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
		demuxInput = args.Get(1).(miscworkflows.DemuxCheckInput)
	}).Return(&miscworkflows.DemuxCheckResult{Outcome: "PASSED", Mailed: true}, nil)

	var qcInput miscworkflows.QScanRawImportInput
	s.env.OnWorkflow(miscworkflows.QScanRawImport, mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
		qcInput = args.Get(1).(miscworkflows.QScanRawImportInput)
	}).Return(nil, nil)
	s.env.OnWorkflow(miscworkflows.TranscodePreviewVX, mock.Anything, mock.Anything).Times(2).Return(nil, nil)
	s.env.OnWorkflow(miscworkflows.TranscribeVX, mock.Anything, mock.Anything).Times(2).Return(nil)

	s.env.ExecuteWorkflow(RawMaterial, params)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	s.Equal(params.Recipients, demuxInput.Recipients)
	if s.Len(demuxInput.Files, 2, "every media file is demux checked, audio included") {
		s.Equal("CLIP_01.mxf", demuxInput.Files[0].Base())
		s.Equal("AUDIO_01.wav", demuxInput.Files[1].Base())
		s.Equal(paths.IsilonDrive, demuxInput.Files[0].Drive)
		s.Contains(demuxInput.Files[0].Path, "Production/raw/", "the file is checked at its final location")
	}

	s.Equal(params.Recipients, qcInput.Recipients)
	if s.Len(qcInput.Files, 1, "the audio file has no video and is not QC'd") {
		s.Equal("VX-VIDEO", qcInput.Files[0].VXID)
		s.Equal("CLIP_01.mxf", qcInput.Files[0].Path.Base())
		s.Equal(paths.IsilonDrive, qcInput.Files[0].Path.Drive)
	}
}

func Test_RawMaterialTestSuite(t *testing.T) {
	suite.Run(t, new(RawMaterialTestSuite))
}
