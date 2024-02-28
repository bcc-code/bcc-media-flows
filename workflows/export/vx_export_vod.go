package export

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"

	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/common/smil"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/bcc-code/bcc-media-platform/backend/asset"
	"github.com/bcc-code/bcc-media-platform/backend/events"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
)

// VXExportToVOD exports the specified vx item to VOD / app.bcc.media
// It will normalize audio, create video files mux them together and upload them to S3
// After this flow, a job will be triggered in the BCC Media Platform to ingest the files
func VXExportToVOD(ctx workflow.Context, params VXExportChildWorkflowParams) (*VXExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportToVOD")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	// We start chapter export and pick the results up later when needed
	var chapterDataWF workflow.Future
	if params.ParentParams.WithChapters {
		chapterDataWF = wfutils.Execute(ctx, vsactivity.GetChapterDataActivity, vsactivity.GetChapterDataParams{
			ExportData: &params.ExportData,
		})
	}

	{
		keys, err := wfutils.GetMapKeysSafely(ctx, params.MergeResult.SubtitleFiles)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			subtitle := params.MergeResult.SubtitleFiles[key]
			err = wfutils.CopyFile(ctx, subtitle, params.OutputDir.Append(subtitle.Base()))
			if err != nil {
				return nil, err
			}
		}
	}

	audioFiles, err := prepareAudioFiles(ctx, params.MergeResult, params.TempDir, true, params.ParentParams.IgnoreSilence)
	if err != nil {
		return nil, err
	}
	audioKeys, err := wfutils.GetMapKeysSafely(ctx, audioFiles)
	if err != nil {
		return nil, err
	}

	var resolutions []vsapi.Resolution
	err = wfutils.Execute(ctx, vsactivity.GetResolutions, vsactivity.GetResolutionsParams{
		VXID: params.ExportData.Clips[0].VXID,
	}).Get(ctx, &resolutions)
	if err != nil {
		return nil, err
	}

	if len(params.ParentParams.Resolutions) > 0 {
		var selectedResolutions []vsapi.Resolution
		for _, i := range params.ParentParams.Resolutions {
			selectedResolutions = append(selectedResolutions, resolutions[i])
		}
		resolutions = selectedResolutions
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
		ingestFolder:           params.ExportData.SafeTitle + "_" + params.RunID,
		params:                 params,
		filesSelector:          workflow.NewSelector(ctx),
		qualitiesWithLanguages: getQualitiesWithLanguages(audioKeys),
	}

	onVideoCreated := func(f workflow.Future, q string) {
		var result common.VideoResult
		err := f.Get(ctx, &result)
		if err != nil {
			logger.Error("Failed to get video result", "error", err)
			service.errs = append(service.errs, err)
			return
		}
		future := createStreamFile(ctx, q, result.OutputPath, params.OutputDir, service.qualitiesWithLanguages, audioFiles)
		onFileCreated := func(f workflow.Future) {
			service.handleStreamWorkflowFuture(ctx, q, f)
		}
		service.filesSelector.AddFuture(future, onFileCreated)
		if params.ParentParams.WithFiles && lo.Contains(fileQualities, q) {
			for _, key := range audioKeys {
				lang := key
				audioPath := audioFiles[lang]

				future := createTranslatedFile(ctx, lang, result.OutputPath, params.OutputDir, audioPath, params.MergeResult.SubtitleFiles)
				onFileCreated := func(f workflow.Future) {
					service.handleFileWorkflowFuture(ctx, lang, q, f)
				}
				service.filesSelector.AddFuture(future, onFileCreated)
			}
		}
	}

	videosByQuality := getVideosByQuality(*params.MergeResult.VideoFile, params.TempDir, wm)
	videoKeys, err := doVideoTasks(ctx, service.filesSelector, videosByQuality, onVideoCreated)
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
		chapterDataWF,
		params.OutputDir)
}

type vxExportVodService struct {
	params                 VXExportChildWorkflowParams
	ingestFolder           string
	qualitiesWithLanguages map[string][]bccmflows.Language
	filesSelector          workflow.Selector
	streams                []smil.Video
	files                  []asset.IngestFileMeta
	tasks                  []workflow.Future
	errs                   []error
}

func prepareAudioFiles(ctx workflow.Context, mergeResult MergeExportDataResult, tempDir paths.Path, normalizeAudio, ignoreSilence bool) (map[string]paths.Path, error) {
	prepareFilesSelector := workflow.NewSelector(ctx)

	if normalizeAudio {
		var silentAudioLanguages []string
		langs, err := wfutils.GetMapKeysSafely(ctx, mergeResult.AudioFiles)
		if err != nil {
			return nil, err
		}
		normalizedFutures := map[string]workflow.Future{}
		// Normalize audio
		for _, lang := range langs {
			audio := mergeResult.AudioFiles[lang]
			future := wfutils.Execute(ctx, activities.NormalizeAudioActivity, activities.NormalizeAudioParams{
				FilePath:              audio,
				TargetLUFS:            -24,
				PerformOutputAnalysis: true,
				OutputPath:            tempDir,
			})
			normalizedFutures[lang] = future
		}

		for _, lang := range langs {
			future := normalizedFutures[lang]
			normalizedRes := activities.NormalizeAudioResult{}
			err := future.Get(ctx, &normalizedRes)
			if err != nil {
				workflow.GetLogger(ctx).Error("Failed to get normalized audio result", "error", err)
				return nil, fmt.Errorf("failed to normalize audio for language %s: %w", lang, err)
			}

			if normalizedRes.IsSilent {
				silentAudioLanguages = append(silentAudioLanguages, lang)
				delete(mergeResult.AudioFiles, lang)
			} else {
				mergeResult.AudioFiles[lang] = normalizedRes.FilePath
			}
		}

		if len(silentAudioLanguages) > 0 && !ignoreSilence {
			return nil, fmt.Errorf("audio for languages `%s` is silent", strings.Join(silentAudioLanguages, ", "))
		}
	}

	var audioFiles = map[string]paths.Path{}
	audioKeys, err := startAudioTasks(ctx, prepareFilesSelector, mergeResult.AudioFiles, tempDir, func(f workflow.Future, l string) {
		var result common.AudioResult
		err := f.Get(ctx, &result)
		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to get audio result", "error", err)
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
	chapterDataWF workflow.Future,
	outputDir paths.Path,
) (*VXExportResult, error) {
	ingestData := asset.IngestJSONMeta{
		Title:    v.params.ExportData.SafeTitle,
		ID:       v.params.ParentParams.VXID,
		Duration: formatSecondsToTimestamp(v.params.MergeResult.Duration),
	}
	var smilData smil.Smil
	smilData.XMLName.Local = "smil"
	smilData.XMLName.Space = "http://www.w3.org/2001/SMIL20/Language"
	smilData.Head.Meta.Name = "formats"
	smilData.Head.Meta.Content = "mp4"

	smilData.Body.Switch.Videos = v.streams
	smilData.Body.Switch.TextStreams = getSubtitlesResult(v.params.MergeResult.SubtitleFiles)

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

	if v.params.Upload {
		// Copies created files and any remaining files needed.
		err = wfutils.Execute(ctx, activities.RcloneCopyDir, activities.RcloneCopyDirInput{
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
		notifyExportDone(ctx, v.params, "vod")
	} else {
		notifyExportDone(ctx, v.params, "isilon")
	}

	//err = DeletePath(ctx, tempFolder)
	return &VXExportResult{
		ID:           v.params.ParentParams.VXID,
		ChaptersFile: ingestData.ChaptersFile,
		SmilFile:     ingestData.SmilFile,
		Duration:     ingestData.Duration,
		Title:        ingestData.Title,
	}, nil
}

func (v *vxExportVodService) handleFileWorkflowFuture(ctx workflow.Context, lang string, q string, f workflow.Future) {
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
		Resolution:    q,
		AudioLanguage: code,
		Mime:          "video/mp4",
		Path:          result.Path.Base(),
	})

	v.copyToIngest(ctx, result.Path)
}

func (v *vxExportVodService) handleStreamWorkflowFuture(ctx workflow.Context, q string, f workflow.Future) {
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
	if !v.params.Upload {
		return
	}
	v.tasks = append(v.tasks, wfutils.Execute(ctx, activities.RcloneCopyFile, activities.RcloneFileInput{
		Source:      path,
		Destination: paths.New(paths.AssetIngestDrive, filepath.Join(v.ingestFolder, path.Base())),
	}))
}
