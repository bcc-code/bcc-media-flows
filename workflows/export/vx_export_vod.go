package export

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bcc-media-platform/backend/asset"
	"github.com/bcc-code/bcc-media-platform/backend/events"
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/common/smil"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"go.temporal.io/sdk/workflow"
)

func VXExportToVOD(ctx workflow.Context, params VXExportChildWorkflowParams) (*VXExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportToVOD")

	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	// We start chapter export and pick the results up later when needed
	var chapterDataWF workflow.Future
	if params.ParentParams.WithChapters {
		chapterDataWF = workflow.ExecuteActivity(ctx, vidispine.GetChapterDataActivity, vidispine.GetChapterDataParams{
			ExportData: &params.ExportData,
		})
	}

	ingestData := asset.IngestJSONMeta{
		Title:    params.ExportData.SafeTitle,
		ID:       params.ParentParams.VXID,
		Duration: formatSecondsToTimestamp(params.MergeResult.Duration),
	}

	var videoFiles map[string]string
	var audioFiles map[string]string
	{
		var result PrepareFilesResult
		ctx = workflow.WithChildOptions(ctx, wfutils.GetDefaultWorkflowOptions())
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
	if chapterDataWF != nil {
		ingestData.ChaptersFile = "chapters.json"
		var chaptersData []asset.Chapter
		err = chapterDataWF.Get(ctx, &chaptersData)
		if err != nil {
			return nil, err
		}
		marshalled, err := json.Marshal(chaptersData)
		if err != nil {
			return nil, err
		}
		err = wfutils.WriteFile(ctx, filepath.Join(params.OutputDir, "chapters.json"), marshalled)
		if err != nil {
			return nil, err
		}
	}

	marshalled, err := json.Marshal(ingestData)
	if err != nil {
		return nil, err
	}

	err = wfutils.WriteFile(ctx, filepath.Join(params.OutputDir, "ingest.json"), marshalled)
	if err != nil {
		return nil, err
	}

	ingestFolder := params.ExportData.SafeTitle + "_" + workflow.GetInfo(ctx).OriginalRunID

	err = workflow.ExecuteActivity(ctx, activities.RcloneCopyDir, activities.RcloneCopyDirInput{
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

	return &VXExportResult{
		ChaptersFile: ingestData.ChaptersFile,
		SmilFile:     ingestData.SmilFile,
		ID:           params.ParentParams.VXID,
		Duration:     ingestData.Duration,
		Title:        ingestData.Title,
	}, nil
}
