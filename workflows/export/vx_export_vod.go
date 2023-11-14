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

	audioFiles, err := prepareAudioFiles(ctx, params.MergeResult, params.TempDir)
	if err != nil {
		return nil, err
	}
	audioKeys, err := wfutils.GetMapKeysSafely(ctx, audioFiles)
	if err != nil {
		return nil, err
	}

	var wm *paths.Path
	if params.ParentParams.WatermarkPath != "" {
		path, err := paths.Parse(params.ParentParams.WatermarkPath)
		if err != nil {
			return nil, err
		}
		wm = &path
	}

	filesSelector := workflow.NewSelector(ctx)
	qualities := getVideoQualities(*params.MergeResult.VideoFile, params.TempDir, wm)
	qualitiesWithLanguages := getQualitiesWithLanguages(audioKeys)

	var uploadTasks []workflow.Future
	var streams []smil.Video
	var files []asset.IngestFileMeta

	ingestFolder := params.ExportData.SafeTitle + "_" + workflow.GetInfo(ctx).OriginalRunID

	var videoFiles = map[quality]paths.Path{}
	videoKeys, err := startVideoTasks(ctx, filesSelector, qualities, func(f workflow.Future, q quality) {
		var result common.VideoResult
		err := f.Get(ctx, &result)
		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to get video result", "error", err)
			return
		}
		videoFiles[q] = result.OutputPath
		if lo.Contains(streamQualities, q) {
			filesSelector.AddFuture(createStreamFile(ctx, q, result.OutputPath, params.OutputDir, qualitiesWithLanguages, audioFiles), func(f workflow.Future) {
				var result common.MuxResult
				err := f.Get(ctx, &result)
				if err != nil {
					workflow.GetLogger(ctx).Error("Failed to get mux result", "error", err)
					return
				}

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
		}
		if params.ParentParams.WithFiles && lo.Contains(fileQualities, q) {
			for _, key := range audioKeys {
				lang := key
				audioPath := audioFiles[lang]
				filesSelector.AddFuture(createTranslatedFile(ctx, lang, result.OutputPath, params.OutputDir, audioPath, params.MergeResult.SubtitleFiles), func(f workflow.Future) {
					var result common.MuxResult
					err := f.Get(ctx, &result)
					if err != nil {
						workflow.GetLogger(ctx).Error("Failed to get mux result", "error", err)
						return
					}
					code := bccmflows.LanguagesByISO[lang].ISO6392TwoLetter
					if code == "" {
						code = lang
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
		}
	})
	if err != nil {
		return nil, err
	}

	for range videoKeys {
		filesSelector.Select(ctx)
	}
	for range qualitiesWithLanguages {
		filesSelector.Select(ctx)
	}

	var smilData smil.Smil
	smilData.XMLName.Local = "smil"
	smilData.XMLName.Space = "http://www.w3.org/2001/SMIL20/Language"
	smilData.Head.Meta.Name = "formats"
	smilData.Head.Meta.Content = "mp4"

	if params.ParentParams.WithFiles {
		for range fileQualities {
			for range audioKeys {
				filesSelector.Select(ctx)
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
	smilData.Body.Switch.TextStreams = getSubtitlesResult(params.MergeResult.SubtitleFiles)

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

func prepareAudioFiles(ctx workflow.Context, mergeResult MergeExportDataResult, tempDir paths.Path) (map[string]paths.Path, error) {
	prepareFilesSelector := workflow.NewSelector(ctx)

	var audioFiles = map[string]paths.Path{}
	audioKeys, err := startAudioTasks(ctx, prepareFilesSelector, mergeResult.AudioFiles, tempDir, func(f workflow.Future, l string) {
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

	return audioFiles, nil
}
