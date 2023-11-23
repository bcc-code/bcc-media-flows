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
	vsactivity "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/common/smil"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/utils/workflows"
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
		chapterDataWF = workflow.ExecuteActivity(ctx, vsactivity.GetChapterDataActivity, vsactivity.GetChapterDataParams{
			ExportData: &params.ExportData,
		})
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

	service := &vxExportVodService{
		ingestFolder:           params.ExportData.SafeTitle + "_" + workflow.GetInfo(ctx).OriginalRunID,
		params:                 params,
		filesSelector:          workflow.NewSelector(ctx),
		qualitiesWithLanguages: getQualitiesWithLanguages(audioKeys),
	}

	videoKeys, err := startVideoTasks(ctx, service.filesSelector, getVideoQualities(*params.MergeResult.VideoFile, params.TempDir, wm), func(f workflow.Future, q quality) {
		var result common.VideoResult
		err := f.Get(ctx, &result)
		if err != nil {
			logger.Error("Failed to get video result", "error", err)
			service.errs = append(service.errs, err)
			return
		}
		if lo.Contains(streamQualities, q) {
			service.filesSelector.AddFuture(createStreamFile(ctx, q, result.OutputPath, params.OutputDir, service.qualitiesWithLanguages, audioFiles), func(f workflow.Future) {
				service.handleStreamWorkflowFuture(ctx, q, f)
			})
		}
		if params.ParentParams.WithFiles && lo.Contains(fileQualities, q) {
			for _, key := range audioKeys {
				lang := key
				audioPath := audioFiles[lang]
				service.filesSelector.AddFuture(createTranslatedFile(ctx, lang, result.OutputPath, params.OutputDir, audioPath, params.MergeResult.SubtitleFiles), func(f workflow.Future) {
					service.handleFileWorkflowFuture(ctx, lang, q, f)
				})
			}
		}
	})
	if err != nil {
		return nil, err
	}

	// Wait for all selector tasks to complete (fills slices, etc.)
	for range videoKeys {
		service.filesSelector.Select(ctx)
	}
	for range service.qualitiesWithLanguages {
		service.filesSelector.Select(ctx)
	}
	if params.ParentParams.WithFiles {
		for range fileQualities {
			for range audioKeys {
				service.filesSelector.Select(ctx)
			}
		}
	}
	for _, task := range service.tasks {
		err = task.Get(ctx, nil)
		if err != nil {
			return nil, err
		}
	}
	for _, err = range service.errs {
		return nil, err
	}

	return service.setMetadataAndPublishToVOD(
		ctx,
		params,
		chapterDataWF,
		params.OutputDir)
}

type vxExportVodService struct {
	params                 VXExportChildWorkflowParams
	ingestFolder           string
	qualitiesWithLanguages map[quality][]bccmflows.Language
	filesSelector          workflow.Selector
	streams                []smil.Video
	files                  []asset.IngestFileMeta
	tasks                  []workflow.Future
	errs                   []error
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

func (v *vxExportVodService) setMetadataAndPublishToVOD(
	ctx workflow.Context,
	params VXExportChildWorkflowParams,
	chapterDataWF workflow.Future,
	outputDir paths.Path,
) (*VXExportResult, error) {
	ingestData := asset.IngestJSONMeta{
		Title:    params.ExportData.SafeTitle,
		ID:       params.ParentParams.VXID,
		Duration: formatSecondsToTimestamp(params.MergeResult.Duration),
	}
	var smilData smil.Smil
	smilData.XMLName.Local = "smil"
	smilData.XMLName.Space = "http://www.w3.org/2001/SMIL20/Language"
	smilData.Head.Meta.Name = "formats"
	smilData.Head.Meta.Content = "mp4"

	smilData.Body.Switch.Videos = v.streams
	smilData.Body.Switch.TextStreams = getSubtitlesResult(params.MergeResult.SubtitleFiles)

	xmlData, _ := xml.MarshalIndent(smilData, "", "\t")
	xmlData = append([]byte("<?xml version=\"1.0\" encoding=\"utf-8\" standalone=\"yes\"?>\n"), xmlData...)
	err := wfutils.WriteFile(ctx, outputDir.Append("aws.smil"), xmlData)
	if err != nil {
		return nil, err
	}

	ingestData.Files = v.files
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
		err = wfutils.WriteFile(ctx, outputDir.Append("chapters.json"), marshalled)
		if err != nil {
			return nil, err
		}
	}

	marshalled, err := json.Marshal(ingestData)
	if err != nil {
		return nil, err
	}

	err = wfutils.WriteFile(ctx, outputDir.Append("ingest.json"), marshalled)
	if err != nil {
		return nil, err
	}

	// Copies created files and any remaining files needed.
	err = workflow.ExecuteActivity(ctx, activities.RcloneCopyDir, activities.RcloneCopyDirInput{
		Source:      outputDir.Rclone(),
		Destination: fmt.Sprintf("s3prod:vod-asset-ingest-prod/" + v.ingestFolder),
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	err = wfutils.PublishEvent(ctx, "asset.delivered", events.AssetDelivered{
		JSONMetaPath: filepath.Join(v.ingestFolder, "ingest.json"),
	})
	if err != nil {
		return nil, err
	}

	//err = DeletePath(ctx, tempFolder)
	return &VXExportResult{
		ID:           params.ParentParams.VXID,
		ChaptersFile: ingestData.ChaptersFile,
		SmilFile:     ingestData.SmilFile,
		Duration:     ingestData.Duration,
		Title:        ingestData.Title,
	}, err
}

func (v *vxExportVodService) handleFileWorkflowFuture(ctx workflow.Context, lang string, q quality, f workflow.Future) {
	logger := workflow.GetLogger(ctx)

	var result common.MuxResult
	err := f.Get(ctx, &result)
	if err != nil {
		logger.Error("Failed to get mux result", "error", err)
		v.errs = append(v.errs, err)
		return
	}
	code := bccmflows.LanguagesByISO[lang].ISO6392TwoLetter
	if code == "" {
		code = lang
	}
	v.files = append(v.files, asset.IngestFileMeta{
		Resolution:    string(q),
		AudioLanguage: code,
		Mime:          "video/mp4",
		Path:          result.Path.Base(),
	})

	v.copyToIngest(ctx, result.Path)
}

func (v *vxExportVodService) handleStreamWorkflowFuture(ctx workflow.Context, q quality, f workflow.Future) {
	logger := workflow.GetLogger(ctx)
	var result common.MuxResult
	err := f.Get(ctx, &result)
	if err != nil {
		logger.Error("Failed to get mux result", "error", err)
		v.errs = append(v.errs, err)
		return
	}

	fileLanguages := v.qualitiesWithLanguages[q]

	v.streams = append(v.streams, smil.Video{
		Src:          result.Path.Base(),
		IncludeAudio: fmt.Sprintf("%t", len(fileLanguages) > 0),
		SystemLanguage: strings.Join(lo.Map(fileLanguages, func(i bccmflows.Language, _ int) string {
			return i.ISO6391
		}), ","),
		AudioName: strings.Join(lo.Map(fileLanguages, func(i bccmflows.Language, _ int) string {
			return i.LanguageNameSystem
		}), ","),
	})

	v.copyToIngest(ctx, result.Path)
}

func (v *vxExportVodService) copyToIngest(ctx workflow.Context, path paths.Path) {
	v.tasks = append(v.tasks, wfutils.ExecuteWithQueue(ctx, activities.RcloneCopyFile, activities.RcloneFileInput{
		Source:      path,
		Destination: paths.New(paths.AssetIngestDrive, filepath.Join(v.ingestFolder, path.Base())),
	}))
}
