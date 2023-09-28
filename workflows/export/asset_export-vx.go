package export

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bcc-media-platform/backend/asset"
	"github.com/bcc-code/bcc-media-platform/backend/events"
	"github.com/orsinium-labs/enum"

	"github.com/bcc-code/bccm-flows/activities"
	avidispine "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/common/smil"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"github.com/bcc-code/bccm-flows/workflows"
	"go.temporal.io/sdk/workflow"
)

type AssetExportDestination enum.Member[string]

var (
	AssetExportDestinationPlayout = AssetExportDestination{"playout"}
	AssetExportDestinationVOD     = AssetExportDestination{"vod"}
	AssetExportDestinationBMM     = AssetExportDestination{"bmm"}
	AssetExportDestinations       = enum.New(
		AssetExportDestinationPlayout,
		AssetExportDestinationVOD,
		AssetExportDestinationBMM,
	)
)

type VXExportParams struct {
	VXID          string
	WithFiles     bool
	WithChapters  bool
	WatermarkPath string
	Destinations  []string
}

type AssetExportResult struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Duration     string `json:"duration"`
	SmilFile     string `json:"smil_file"`
	ChaptersFile string `json:"chapters_file"`
}

const (
	r1080p = "1920x1080"
	r720p  = "1280x720"
	r540p  = "960x540"
	r360p  = "640x360"
	r270p  = "480x270"
	r180p  = "320x180"
)

func formatSecondsToTimestamp(seconds float64) string {
	hours := int(seconds / 3600)
	seconds -= float64(hours * 3600)

	minutes := int(seconds / 60)
	seconds -= float64(minutes * 60)

	secondsInt := int(seconds)

	return fmt.Sprintf("%02d:%02d:%02d:00", hours, minutes, secondsInt)
}

func VXExport(ctx workflow.Context, params VXExportParams) ([]workflows.ResultOrError[AssetExportResult], error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting VXExport")

	options := workflows.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	var data *vidispine.ExportData
	err := workflow.ExecuteActivity(ctx, avidispine.GetExportDataActivity, avidispine.GetExportDataParams{
		VXID: params.VXID,
	}).Get(ctx, &data)
	if err != nil {
		return nil, err
	}

	logger.Info("Retrieved data from vidispine")

	tempDir, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	outputDir := filepath.Join(tempDir, "output")
	err = wfutils.CreateFolder(ctx, outputDir)
	if err != nil {
		return nil, err
	}

	ctx = workflow.WithChildOptions(ctx, workflows.GetDefaultWorkflowOptions())

	var mergeResult MergeExportDataResult
	err = workflow.ExecuteChildWorkflow(ctx, MergeExportData, MergeExportDataParams{
		ExportData:   data,
		TempDir:      tempDir,
		SubtitlesDir: outputDir,
	}).Get(ctx, &mergeResult)
	if err != nil {
		return nil, err
	}

	// Destination branching:  VOD, playout, bmm, etc.
	var resultFutures []workflow.Future
	for _, dest := range params.Destinations {
		destination := AssetExportDestinations.Parse(dest)
		if destination == nil {
			return nil, fmt.Errorf("invalid destination: %s", dest)
		}

		var w interface{}
		switch *destination {
		case AssetExportDestinationVOD:
			w = VXExportToVOD
		case AssetExportDestinationPlayout:
			w = VXExportToPlayout
		default:
			return nil, fmt.Errorf("destination not implemented: %s", dest)
		}

		ctx = workflow.WithChildOptions(ctx, workflows.GetDefaultWorkflowOptions())
		future := workflow.ExecuteChildWorkflow(ctx, w, VXExportChildWorklowParams{
			ParentParams: params,
			ExportData:   *data,
			MergeResult:  mergeResult,
			TempDir:      tempDir,
			OutputDir:    outputDir,
		})
		if err != nil {
			return nil, err
		}
		resultFutures = append(resultFutures, future)
	}

	results := []workflows.ResultOrError[AssetExportResult]{}
	for _, future := range resultFutures {
		var result *AssetExportResult
		err = future.Get(ctx, &result)
		results = append(results, workflows.ResultOrError[AssetExportResult]{
			Result: result,
			Error:  err,
		})
	}

	return results, nil
}

type VXExportChildWorklowParams struct {
	ParentParams VXExportParams       `json:"parent_params"`
	ExportData   vidispine.ExportData `json:"export_data"`
	MergeResult  MergeExportDataResult
	TempDir      string
	OutputDir    string
}

func VXExportToPlayout(ctx workflow.Context, params VXExportChildWorklowParams) (*AssetExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportToPlayout")

	options := workflows.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	xdcamOutputDir := filepath.Join(params.TempDir, "xdcam_output")
	err := wfutils.CreateFolder(ctx, xdcamOutputDir)
	if err != nil {
		return nil, err
	}

	// Transcode video using playout encoding
	var videoResult common.VideoResult
	err = workflow.ExecuteActivity(ctx, activities.TranscodeToXDCAMActivity, activities.EncodeParams{
		Bitrate:    "50M",
		FilePath:   params.MergeResult.VideoFile,
		OutputDir:  xdcamOutputDir,
		Resolution: r1080p,
		FrameRate:  25,
	}).Get(ctx, &videoResult)
	if err != nil {
		return nil, err
	}

	// Mux into MXF file with 16 audio channels
	var muxResult *common.PlayoutMuxResult
	err = workflow.ExecuteActivity(ctx, activities.TranscodePlayoutMux, common.PlayoutMuxInput{
		VideoFilePath:     videoResult.OutputPath,
		AudioFilePaths:    params.MergeResult.AudioFiles,
		SubtitleFilePaths: params.MergeResult.SubtitleFiles,
		OutputDir:         params.OutputDir,
		FallbackLanguage:  "nor",
	}).Get(ctx, &muxResult)

	if err != nil {
		return nil, err
	}

	// Rclone to playout
	source := strings.Replace(muxResult.Path, utils.GetIsilonPrefix()+"/", "isilon:isilon/", 1)
	destination := "playout:/dropbox" + filepath.Base(muxResult.Path)
	err = workflow.ExecuteActivity(ctx, activities.RcloneCopy, activities.RcloneCopyInput{
		Source:      source,
		Destination: destination,
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &AssetExportResult{
		ID:       params.ParentParams.VXID,
		Title:    params.ExportData.Title,
		Duration: formatSecondsToTimestamp(params.MergeResult.Duration),
	}, nil
}

func VXExportToVOD(ctx workflow.Context, params VXExportChildWorklowParams) (*AssetExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportToVOD")

	options := workflows.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	// We start chapter export and pick the results up later when needed
	var chapterDataWF workflow.Future
	if params.ParentParams.WithChapters {
		chapterDataWF = workflow.ExecuteActivity(ctx, avidispine.GetChapterDataActivity, avidispine.GetChapterDataParams{
			ExportData: &params.ExportData,
		})
	}

	ingestData := asset.IngestJSONMeta{
		Title:    params.ExportData.Title,
		ID:       params.ParentParams.VXID,
		Duration: formatSecondsToTimestamp(params.MergeResult.Duration),
	}

	var videoFiles map[string]string
	var audioFiles map[string]string
	{
		var result PrepareFilesResult
		ctx = workflow.WithChildOptions(ctx, workflows.GetDefaultWorkflowOptions())
		err := workflow.ExecuteChildWorkflow(ctx, PrepareFiles, PrepareFilesParams{
			OutputPath:    params.TempDir,
			VideoFile:     params.MergeResult.VideoFile,
			AudioFiles:    params.MergeResult.AudioFiles,
			WatermarkPath: params.ParentParams.WatermarkPath,
		}).Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		videoFiles = result.VideoFiles
		audioFiles = result.AudioFiles
	}

	subtitleFiles := params.MergeResult.SubtitleFiles

	var smilData smil.Smil
	smilData.XMLName.Local = "smil"
	smilData.XMLName.Space = "http://www.w3.org/2001/SMIL20/Language"
	smilData.Head.Meta.Name = "formats"
	smilData.Head.Meta.Content = "mp4"

	{
		var result *MuxFilesResult
		err := workflow.ExecuteChildWorkflow(ctx, MuxFiles, MuxFilesParams{
			VideoFiles:    videoFiles,
			AudioFiles:    audioFiles,
			SubtitleFiles: subtitleFiles,
			OutputPath:    params.OutputDir,
			WithFiles:     params.ParentParams.WithFiles,
		}).Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		ingestData.Files = result.Files
		smilData.Body.Switch.Videos = result.Streams
		smilData.Body.Switch.TextStreams = result.Subtitles
	}

	xmlData, _ := xml.MarshalIndent(smilData, "", "\t")
	xmlData = append([]byte("<?xml version=\"1.0\" encoding=\"utf-8\" standalone=\"yes\"?>\n"), xmlData...)
	err := wfutils.WriteFile(ctx, filepath.Join(params.OutputDir, "aws.smil"), xmlData)
	if err != nil {

		return nil, err
	}

	ingestData.SmilFile = "aws.smil"

	marshalled, err := json.Marshal(ingestData)
	if err != nil {
		return nil, err
	}

	err = wfutils.WriteFile(ctx, filepath.Join(params.OutputDir, "ingest.json"), marshalled)
	if err != nil {
		return nil, err
	}

	if chapterDataWF != nil {
		ingestData.ChaptersFile = "chapters.json"
		var chaptersData []asset.Chapter
		err = chapterDataWF.Get(ctx, &chaptersData)
		if err != nil {
			return nil, err
		}
		marshalled, err = json.Marshal(chaptersData)
		if err != nil {
			return nil, err
		}
		err = wfutils.WriteFile(ctx, filepath.Join(params.OutputDir, "chapters.json"), marshalled)
		if err != nil {
			return nil, err
		}
	}

	ingestFolder := params.ExportData.Title + "_" + workflow.GetInfo(ctx).OriginalRunID

	err = workflow.ExecuteActivity(ctx, activities.RcloneCopy, activities.RcloneCopyInput{
		Source:      strings.Replace(params.OutputDir, utils.GetIsilonPrefix()+"/", "isilon:isilon/", 1),
		Destination: fmt.Sprintf("s3prod:vod-asset-ingest-prod/" + ingestFolder),
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	err = wfutils.PublishEvent(ctx, "asset.delivered", events.AssetDelivered{
		JSONMetaPath: filepath.Join(ingestFolder, "ingest.json"),
	})
	if err != nil {
		return nil, err
	}

	//err = DeletePath(ctx, tempFolder)

	return &AssetExportResult{
		ChaptersFile: ingestData.ChaptersFile,
		SmilFile:     ingestData.SmilFile,
		ID:           params.ParentParams.VXID,
		Duration:     ingestData.Duration,
		Title:        ingestData.Title,
	}, nil
}
