package export

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bcc-media-platform/backend/asset"
	"github.com/bcc-code/bcc-media-platform/backend/events"
	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/common/smil"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"github.com/samber/lo"
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

	prepareFilesSelector := workflow.NewSelector(ctx)
	var wm *paths.Path
	if params.ParentParams.WatermarkPath != "" {
		path, err := paths.Parse(params.ParentParams.WatermarkPath)
		if err != nil {
			return nil, err
		}
		wm = &path
	}

	qualities := getVideoQualities(*params.MergeResult.VideoFile, params.TempDir, wm)

	var videoFiles = map[quality]paths.Path{}
	videoKeys, err := startVideoTasks(ctx, prepareFilesSelector, qualities, func(f workflow.Future, q quality) {
		var result common.VideoResult
		err := f.Get(ctx, &result)
		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to get video result", "error", err)
			return
		}
		videoFiles[q] = result.OutputPath
	})

	if err != nil {
		return nil, err
	}

	var audioFiles = map[string]paths.Path{}
	audioKeys, err := startAudioTasks(ctx, prepareFilesSelector, params.MergeResult.AudioFiles, params.TempDir, func(f workflow.Future, l string) {
		var result common.AudioResult
		err := f.Get(ctx, &result)
		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to get video result", "error", err)
			return
		}
		audioFiles[l] = result.OutputPath
	})
	if err != nil {
		return nil, err
	}

	for range audioKeys {
		prepareFilesSelector.Select(ctx)
	}
	for range videoKeys {
		prepareFilesSelector.Select(ctx)
	}

	ingestFolder := params.ExportData.SafeTitle + "_" + workflow.GetInfo(ctx).OriginalRunID

	var smilData smil.Smil
	smilData.XMLName.Local = "smil"
	smilData.XMLName.Space = "http://www.w3.org/2001/SMIL20/Language"
	smilData.Head.Meta.Name = "formats"
	smilData.Head.Meta.Content = "mp4"

	muxParams := MuxFilesParams{
		VideoFiles:    videoFiles,
		AudioFiles:    audioFiles,
		SubtitleFiles: params.MergeResult.SubtitleFiles,
		OutputPath:    params.OutputDir,
		WithFiles:     params.ParentParams.WithFiles,
	}
	qualitiesWithLanguages := getQualitiesWithLanguages(muxParams)
	muxSelector := workflow.NewSelector(ctx)

	var uploadTasks []workflow.Future

	var streams []smil.Video
	startStreamTasks(ctx, muxParams, qualitiesWithLanguages, muxSelector, func(result common.MuxResult, q quality) {
		fileLanguages := qualitiesWithLanguages[q]

		streams = append(streams, smil.Video{
			Src:          result.Path.Base(),
			IncludeAudio: fmt.Sprintf("%t", len(fileLanguages) > 0),
			SystemLanguage: strings.Join(lo.Map(fileLanguages, func(i bccmflows.Language, _ int) string {
				return i.ISO6391
			}), ","),
			AudioName: strings.Join(lo.Map(fileLanguages, func(i bccmflows.Language, _ int) string {
				return i.LanguageNameSystem
			}), ","),
		})

		uploadTasks = append(uploadTasks, wfutils.ExecuteWithQueue(ctx, activities.RcloneCopyFile, activities.RcloneFileInput{
			Source:      result.Path,
			Destination: paths.New(paths.AssetIngestDrive, filepath.Join(ingestFolder, result.Path.Base())),
		}))
	})

	audioLanguages := utils.LanguageKeysToOrderedLanguages(lo.Keys(muxParams.AudioFiles))
	var files []asset.IngestFileMeta
	if muxParams.WithFiles {
		startFileTasks(ctx, muxParams, audioLanguages, muxSelector, func(result common.MuxResult, l string, q quality) {
			code := bccmflows.LanguagesByISO[l].ISO6392TwoLetter
			if code == "" {
				code = l
			}
			files = append(files, asset.IngestFileMeta{
				Resolution:    string(q),
				AudioLanguage: code,
				Mime:          "video/mp4",
				Path:          result.Path.Base(),
			})

			uploadTasks = append(uploadTasks, wfutils.ExecuteWithQueue(ctx, activities.RcloneCopyFile, activities.RcloneFileInput{
				Source:      result.Path,
				Destination: paths.New(paths.AssetIngestDrive, filepath.Join(ingestFolder, result.Path.Base())),
			}))
		})
	}

	for range qualitiesWithLanguages {
		muxSelector.Select(ctx)
	}

	if muxParams.WithFiles {
		for range audioLanguages {
			for range fileQualities {
				muxSelector.Select(ctx)
			}
		}
	}

	for _, task := range uploadTasks {
		err = task.Get(ctx, nil)
		if err != nil {
			return nil, err
		}
	}

	ingestData.Files = files
	smilData.Body.Switch.Videos = streams
	smilData.Body.Switch.TextStreams = getSubtitlesResult(muxParams)

	xmlData, _ := xml.MarshalIndent(smilData, "", "\t")
	xmlData = append([]byte("<?xml version=\"1.0\" encoding=\"utf-8\" standalone=\"yes\"?>\n"), xmlData...)
	err = wfutils.WriteFile(ctx, params.OutputDir.Append("aws.smil"), xmlData)
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
		err = wfutils.WriteFile(ctx, params.OutputDir.Append("chapters.json"), marshalled)
		if err != nil {
			return nil, err
		}
	}

	marshalled, err := json.Marshal(ingestData)
	if err != nil {
		return nil, err
	}

	err = wfutils.WriteFile(ctx, params.OutputDir.Append("ingest.json"), marshalled)
	if err != nil {
		return nil, err
	}

	err = workflow.ExecuteActivity(ctx, activities.RcloneCopyDir, activities.RcloneCopyDirInput{
		Source:      params.OutputDir.Rclone(),
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
