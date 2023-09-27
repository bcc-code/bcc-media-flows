package export

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"github.com/bcc-code/bccm-flows/workflows"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bcc-media-platform/backend/asset"

	"github.com/bcc-code/bccm-flows/activities"
	avidispine "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/common/smil"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/utils"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"go.temporal.io/sdk/workflow"
)

type AssetExportParams struct {
	VXID          string
	WithFiles     bool
	WithChapters  bool
	WatermarkPath string
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

func AssetExportVX(ctx workflow.Context, params AssetExportParams) (*AssetExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting AssetExport")

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

	tempFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	outputFolder := filepath.Join(tempFolder, "output")
	err = wfutils.CreateFolder(ctx, outputFolder)
	if err != nil {
		return nil, err
	}

	ctx = workflow.WithChildOptions(ctx, workflows.GetDefaultWorkflowOptions())

	var mergeResult MergeExportDataResult
	err = workflow.ExecuteChildWorkflow(ctx, MergeExportData, MergeExportDataParams{
		ExportData: data,
		TempPath:   tempFolder,
		OutputPath: outputFolder,
	}).Get(ctx, &mergeResult)
	if err != nil {
		return nil, err
	}

	// We start chapter export and pick the results up later when needed
	var chapterDataWF workflow.Future
	if params.WithChapters {
		chapterDataWF = workflow.ExecuteActivity(ctx, avidispine.GetChapterDataActivity, avidispine.GetChapterDataParams{
			ExportData: data,
		})
	}

	ingestData := asset.IngestJSONMeta{
		Title:    data.Title,
		ID:       params.VXID,
		Duration: formatSecondsToTimestamp(mergeResult.Duration),
	}

	var videoFiles map[string]string
	var audioFiles map[string]string
	{
		var result PrepareFilesResult
		ctx = workflow.WithChildOptions(ctx, workflows.GetDefaultWorkflowOptions())
		err = workflow.ExecuteChildWorkflow(ctx, PrepareFiles, PrepareFilesParams{
			OutputPath:    tempFolder,
			VideoFile:     mergeResult.VideoFile,
			AudioFiles:    mergeResult.AudioFiles,
			WatermarkPath: params.WatermarkPath,
		}).Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		videoFiles = result.VideoFiles
		audioFiles = result.AudioFiles
	}

	subtitleFiles := mergeResult.SubtitleFiles

	var smilData smil.Smil
	smilData.XMLName.Local = "smil"
	smilData.XMLName.Space = "http://www.w3.org/2001/SMIL20/Language"
	smilData.Head.Meta.Name = "formats"
	smilData.Head.Meta.Content = "mp4"

	{
		var result *MuxFilesResult
		err = workflow.ExecuteChildWorkflow(ctx, MuxFiles, MuxFilesParams{
			VideoFiles:    videoFiles,
			AudioFiles:    audioFiles,
			SubtitleFiles: subtitleFiles,
			OutputPath:    outputFolder,
			WithFiles:     params.WithFiles,
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
	err = wfutils.WriteFile(ctx, filepath.Join(outputFolder, "aws.smil"), xmlData)
	if err != nil {

		return nil, err
	}

	ingestData.SmilFile = "aws.smil"

	marshalled, err := json.Marshal(ingestData)
	if err != nil {
		return nil, err
	}

	err = wfutils.WriteFile(ctx, filepath.Join(outputFolder, "ingest.json"), marshalled)
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
		err = wfutils.WriteFile(ctx, filepath.Join(outputFolder, "chapters.json"), marshalled)
		if err != nil {
			return nil, err
		}
	}

	ingestFolder := data.Title + "_" + workflow.GetInfo(ctx).OriginalRunID

	err = workflow.ExecuteActivity(ctx, activities.RcloneUploadDir, activities.RcloneUploadDirInput{
		Source:      strings.Replace(outputFolder, utils.GetIsilonPrefix()+"/", "isilon:isilon/", 1),
		Destination: fmt.Sprintf("s3prod:vod-asset-ingest-prod/" + ingestFolder),
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	event := cloudevents.NewEvent()
	event.SetID(uuid.NewString())
	event.SetSpecVersion(cloudevents.VersionV1)
	event.SetSource("bccm-flows")
	event.SetType("asset.delivered")
	type r struct {
		JSONMetaPath string `json:"jsonMetaPath"`
	}
	err = event.SetData(
		cloudevents.ApplicationJSON,
		r{
			JSONMetaPath: filepath.Join(ingestFolder, "ingest.json"),
		},
	)
	if err != nil {
		return nil, err
	}

	err = workflow.ExecuteActivity(ctx, activities.PubsubPublish, event).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	//err = DeletePath(ctx, tempFolder)

	return &AssetExportResult{
		ChaptersFile: ingestData.ChaptersFile,
		SmilFile:     ingestData.SmilFile,
		ID:           params.VXID,
		Duration:     ingestData.Duration,
		Title:        ingestData.Title,
	}, nil
}
