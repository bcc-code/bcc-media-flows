package vb_export

import (
	"os"
	"testing"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type VBExportTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *VBExportTestSuite) SetupTest() {
	os.Setenv("TEMPORAL_DEBUG", "true")
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *VBExportTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *VBExportTestSuite) Test_VBExport_EmptyVXID() {
	s.env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).Maybe().Return(nil, nil)

	s.env.ExecuteWorkflow(VBExport, VBExportParams{
		VXID:         "",
		Destinations: []string{"xdcam"},
	})
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)
	s.Contains(err.Error(), "vxid is required")
}

func (s *VBExportTestSuite) Test_VBExport_InvalidDestination() {
	s.env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).Maybe().Return(nil, nil)

	s.env.ExecuteWorkflow(VBExport, VBExportParams{
		VXID:         "VX-123",
		Destinations: []string{"nonexistent"},
	})
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)
	s.Contains(err.Error(), "invalid destination")
}

func (s *VBExportTestSuite) Test_VBExport_NoShapes() {
	s.env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).Maybe().Return(nil, nil)
	s.env.OnActivity(activities.Vidispine.GetShapes, mock.Anything, vsactivity.VXOnlyParam{
		VXID: "VX-123",
	}).Return(&vsapi.ShapeResult{
		Shape: []vsapi.Shape{},
	}, nil)

	s.env.ExecuteWorkflow(VBExport, VBExportParams{
		VXID:         "VX-123",
		Destinations: []string{"xdcam"},
	})
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)
	s.Contains(err.Error(), "no clips found")
}

func (s *VBExportTestSuite) Test_VBExport_NoOriginalShape() {
	s.env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).Maybe().Return(nil, nil)
	s.env.OnActivity(activities.Vidispine.GetShapes, mock.Anything, vsactivity.VXOnlyParam{
		VXID: "VX-123",
	}).Return(&vsapi.ShapeResult{
		Shape: []vsapi.Shape{
			{Tag: []string{"preview"}},
		},
	}, nil)

	s.env.ExecuteWorkflow(VBExport, VBExportParams{
		VXID:         "VX-123",
		Destinations: []string{"xdcam"},
	})
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Error(err)
	s.Contains(err.Error(), "no original shape found")
}

func (s *VBExportTestSuite) Test_VBExport_XDCAM_Success() {
	s.env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).Maybe().Return(nil, nil)
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Maybe().Return(nil, nil)

	videoPath := paths.MustParse("/mnt/isilon/Production/masters/test_video.mxf")

	s.env.OnActivity(activities.Vidispine.GetShapes, mock.Anything, vsactivity.VXOnlyParam{
		VXID: "VX-123",
	}).Return(&vsapi.ShapeResult{
		Shape: []vsapi.Shape{
			{
				Tag: []string{"original"},
				ContainerComponent: vsapi.ContainerComponent{
					File: []vsapi.File{
						{URI: []string{"file://" + videoPath.Local()}},
					},
				},
			},
		},
	}, nil)

	s.env.OnActivity(activities.Audio.AnalyzeFile, mock.Anything, mock.Anything).Return(&ffmpeg.StreamInfo{
		HasAudio:     true,
		HasVideo:     true,
		FrameRate:    25,
		Width:        1920,
		Height:       1080,
		TotalSeconds: 60,
		AudioStreams: []ffmpeg.FFProbeStream{
			{Channels: 2, ChannelLayout: "stereo", CodecType: "audio"},
		},
		VideoStreams: []ffmpeg.FFProbeStream{
			{CodecName: "prores", Width: 1920, Height: 1080, CodecType: "video"},
		},
	}, nil)

	s.env.OnActivity(activities.Audio.NormalizeAudioActivity, mock.Anything, mock.Anything).Return(&activities.NormalizeAudioResult{
		FilePath: videoPath,
	}, nil)

	s.env.OnWorkflow(VBExportToXDCAM, mock.Anything, mock.Anything).Return(&VBExportResult{
		ID: "VX-123",
	}, nil)

	s.env.ExecuteWorkflow(VBExport, VBExportParams{
		VXID:         "VX-123",
		Destinations: []string{"xdcam"},
	})
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.NoError(err)

	var results []wfutils.ResultOrError[VBExportResult]
	s.env.GetWorkflowResult(&results)
	s.Len(results, 1)
	s.NotNil(results[0].Result)
	s.Equal("VX-123", results[0].Result.ID)
}

func (s *VBExportTestSuite) Test_VBExportToXDCAM() {
	s.env.OnActivity(activities.Util.SendTelegramMessage, mock.Anything, mock.Anything).Maybe().Return(nil, nil)
	s.env.OnActivity(activities.Util.CreateFolder, mock.Anything, mock.Anything).Maybe().Return(nil, nil)
	s.env.OnActivity(activities.Util.RcloneCheckFileExists, mock.Anything, mock.Anything).Maybe().Return(false, nil)

	outputPath := paths.MustParse("/mnt/temp/workflows/xdcam_output/test_video.mxf")
	s.env.OnActivity(activities.Video.TranscodeToXDCAMActivity, mock.Anything, mock.Anything).Return(&activities.EncodeResult{
		OutputPath: outputPath,
	}, nil)

	s.env.OnActivity(activities.Util.RcloneCopyFile, mock.Anything, mock.Anything).Maybe().Return(1, nil)
	s.env.OnActivity(activities.Util.RcloneWaitForJob, mock.Anything, mock.Anything).Maybe().Return(true, nil)

	s.env.ExecuteWorkflow(VBExportToXDCAM, VBExportChildWorkflowParams{
		ParentParams: VBExportParams{
			VXID:         "VX-123",
			Destinations: []string{"xdcam"},
		},
		InputFile:                  paths.MustParse("/mnt/temp/workflows/test_video.mxf"),
		OriginalFilenameWithoutExt: "test_video",
		TempDir:                    paths.MustParse("/mnt/temp/workflows"),
		OutputDir:                  paths.MustParse("/mnt/temp/workflows/output/xdcam"),
		AnalyzeResult: ffmpeg.StreamInfo{
			HasVideo:  true,
			FrameRate: 25,
		},
	})
	s.True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.NoError(err)

	var result VBExportResult
	s.env.GetWorkflowResult(&result)
	s.Equal("VX-123", result.ID)
}

func TestVBExportTestSuite(t *testing.T) {
	suite.Run(t, new(VBExportTestSuite))
}
