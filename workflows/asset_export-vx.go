package workflows

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/bcc-code/bcc-media-platform/backend/asset"
	"path/filepath"
	"strings"

	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/bcc-code/bccm-flows/activities"
	avidispine "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/common/smil"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/utils"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"github.com/samber/lo"
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

func exportDataToMergeInputs(data *vidispine.ExportData, tempFolder, subtitlesFolder string) (
	mergeInput common.MergeInput,
	audioMergeInputs map[string]*common.MergeInput,
	subtitleMergeInputs map[string]*common.MergeInput,
) {
	mergeInput = common.MergeInput{
		Title:     data.Title,
		OutputDir: tempFolder,
		WorkDir:   tempFolder,
	}

	audioMergeInputs = map[string]*common.MergeInput{}
	subtitleMergeInputs = map[string]*common.MergeInput{}

	for _, clip := range data.Clips {
		mergeInput.Duration += clip.OutSeconds - clip.InSeconds
		mergeInput.Items = append(mergeInput.Items, common.MergeInputItem{
			Path:  clip.VideoFile,
			Start: clip.InSeconds,
			End:   clip.OutSeconds,
		})

		for lan, af := range clip.AudioFiles {
			if _, ok := audioMergeInputs[lan]; !ok {
				audioMergeInputs[lan] = &common.MergeInput{
					Title:     data.Title + "-" + lan,
					OutputDir: tempFolder,
					WorkDir:   tempFolder,
				}
			}

			audioMergeInputs[lan].Duration += clip.OutSeconds - clip.InSeconds
			audioMergeInputs[lan].Items = append(audioMergeInputs[lan].Items, common.MergeInputItem{
				Path:    af.File,
				Start:   clip.InSeconds,
				End:     clip.OutSeconds,
				Streams: af.Streams,
			})
		}

		for lan, sf := range clip.SubtitleFiles {
			if _, ok := subtitleMergeInputs[lan]; !ok {
				subtitleMergeInputs[lan] = &common.MergeInput{
					Title:     data.Title + "-" + lan,
					OutputDir: subtitlesFolder,
					WorkDir:   tempFolder,
				}
			}

			subtitleMergeInputs[lan].Duration += clip.OutSeconds - clip.InSeconds
			subtitleMergeInputs[lan].Items = append(subtitleMergeInputs[lan].Items, common.MergeInputItem{
				Path:  sf,
				Start: clip.InSeconds,
				End:   clip.OutSeconds,
			})
		}
	}

	return
}

func AssetExportVX(ctx workflow.Context, params AssetExportParams) (*AssetExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting AssetExport")

	options := GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	var data *vidispine.ExportData

	err := workflow.ExecuteActivity(ctx, avidispine.GetExportDataActivity, avidispine.GetExportDataParams{
		VXID: params.VXID,
	}).Get(ctx, &data)
	if err != nil {
		return nil, err
	}

	logger.Info("Retrieved data from vidispine")

	tempFolder, err := getWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	outputFolder := filepath.Join(tempFolder, "output")
	err = createFolder(ctx, outputFolder)
	if err != nil {
		return nil, err
	}

	ctx = workflow.WithChildOptions(ctx, GetDefaultWorkflowOptions())

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
		ctx = workflow.WithChildOptions(ctx, GetDefaultWorkflowOptions())
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
	err = writeFile(ctx, filepath.Join(outputFolder, "aws.smil"), xmlData)
	if err != nil {

		return nil, err
	}

	ingestData.SmilFile = "aws.smil"

	marshalled, err := json.Marshal(ingestData)
	if err != nil {
		return nil, err
	}

	err = writeFile(ctx, filepath.Join(outputFolder, "ingest.json"), marshalled)
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
		err = writeFile(ctx, filepath.Join(outputFolder, "chapters.json"), marshalled)
		if err != nil {
			return nil, err
		}
	}

	ingestFolder := data.Title + "_" + workflow.GetInfo(ctx).OriginalRunID

	err = workflow.ExecuteActivity(ctx, activities.RcloneUploadDir, activities.RcloneUploadDirInput{
		Source:      strings.Replace(outputFolder, "/mnt/isilon/", "isilon:isilon/", 1),
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

	//err = deletePath(ctx, tempFolder)

	return &AssetExportResult{
		ChaptersFile: ingestData.ChaptersFile,
		SmilFile:     ingestData.SmilFile,
		ID:           params.VXID,
		Duration:     ingestData.Duration,
		Title:        ingestData.Title,
	}, nil
}

type MergeExportDataResult struct {
	Duration      float64
	VideoFile     string
	AudioFiles    map[string]string
	SubtitleFiles map[string]string
}

type MergeExportDataParams struct {
	ExportData *vidispine.ExportData
	OutputPath string
	TempPath   string
}

func MergeExportData(ctx workflow.Context, params MergeExportDataParams) (*MergeExportDataResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting MergeExportData")
	data := params.ExportData

	mergeInput, audioMergeInputs, subtitleMergeInputs := exportDataToMergeInputs(data, params.TempPath, params.OutputPath)

	options := GetDefaultActivityOptions()
	options.TaskQueue = utils.GetTranscodeQueue()
	ctx = workflow.WithActivityOptions(ctx, options)
	videoTask := workflow.ExecuteActivity(ctx, activities.TranscodeMergeVideo, mergeInput)

	var audioTasks = map[string]workflow.Future{}
	for lang, mi := range audioMergeInputs {
		audioTasks[lang] = workflow.ExecuteActivity(ctx, activities.TranscodeMergeAudio, *mi)
	}

	var subtitleTasks = map[string]workflow.Future{}
	for lang, mi := range subtitleMergeInputs {
		subtitleTasks[lang] = workflow.ExecuteActivity(ctx, activities.TranscodeMergeSubtitles, *mi)
	}

	var videoFile string
	{
		var result common.MergeResult
		err := videoTask.Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		videoFile = result.Path
	}

	var audioFiles = map[string]string{}
	for lang, task := range audioTasks {
		var result common.MergeResult
		err := task.Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		audioFiles[lang] = result.Path
	}

	var subtitleFiles = map[string]string{}
	for lang, task := range subtitleTasks {
		var result common.MergeResult
		err := task.Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		subtitleFiles[lang] = result.Path
	}

	return &MergeExportDataResult{
		Duration:      mergeInput.Duration,
		VideoFile:     videoFile,
		AudioFiles:    audioFiles,
		SubtitleFiles: subtitleFiles,
	}, nil
}

type PrepareFilesParams struct {
	OutputPath    string
	VideoFile     string
	WatermarkPath string
	AudioFiles    map[string]string
}

type PrepareFilesResult struct {
	VideoFiles map[string]string
	AudioFiles map[string]string
}

func PrepareFiles(ctx workflow.Context, params PrepareFilesParams) (*PrepareFilesResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting PrepareFiles")

	options := GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	ctx = workflow.WithTaskQueue(ctx, utils.GetTranscodeQueue())
	tempFolder := params.OutputPath

	var videoTasks = map[string]workflow.Future{}
	{
		videoFile := params.VideoFile
		qualities := map[string]common.VideoInput{
			r1080p: {
				Path:            videoFile,
				Width:           1920,
				Height:          1080,
				Bitrate:         "5M",
				DestinationPath: tempFolder,
				WatermarkPath:   params.WatermarkPath,
			},
			r720p: {
				Path:            videoFile,
				Width:           1280,
				Height:          720,
				Bitrate:         "3M",
				DestinationPath: tempFolder,
				WatermarkPath:   params.WatermarkPath,
			},
			r540p: {
				Path:            videoFile,
				FrameRate:       25,
				Width:           960,
				Height:          540,
				Bitrate:         "1900k",
				DestinationPath: tempFolder,
				WatermarkPath:   params.WatermarkPath,
			},
			r360p: {
				Path:            videoFile,
				FrameRate:       25,
				Width:           640,
				Height:          360,
				Bitrate:         "980k",
				DestinationPath: tempFolder,
				WatermarkPath:   params.WatermarkPath,
			},
			r270p: {
				Path:            videoFile,
				FrameRate:       25,
				Width:           480,
				Height:          270,
				Bitrate:         "610k",
				DestinationPath: tempFolder,
				WatermarkPath:   params.WatermarkPath,
			},
			r180p: {
				Path:            videoFile,
				FrameRate:       25,
				Width:           320,
				Height:          180,
				Bitrate:         "320k",
				DestinationPath: tempFolder,
				WatermarkPath:   params.WatermarkPath,
			},
		}

		for key := range qualities {
			input := qualities[key]
			videoTasks[key] = workflow.ExecuteActivity(ctx, activities.TranscodeToVideoH264, input)
		}
	}

	var audioTasks = map[string]workflow.Future{}
	for lang, path := range params.AudioFiles {
		audioTasks[lang] = workflow.ExecuteActivity(ctx, activities.TranscodeToAudioAac, common.AudioInput{
			Path:            path,
			Bitrate:         "190k",
			DestinationPath: tempFolder,
		})
	}

	var audioFiles = map[string]string{}
	for lang, task := range audioTasks {
		var result common.AudioResult
		err := task.Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		audioFiles[lang] = result.OutputPath
	}

	var videoFiles = map[string]string{}
	for key, task := range videoTasks {
		var result common.VideoResult
		err := task.Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		videoFiles[key] = result.OutputPath
	}

	return &PrepareFilesResult{
		VideoFiles: videoFiles,
		AudioFiles: audioFiles,
	}, nil
}

type MuxFilesParams struct {
	VideoFiles    map[string]string
	AudioFiles    map[string]string
	SubtitleFiles map[string]string
	OutputPath    string
	WithFiles     bool
}

type MuxFilesResult struct {
	Files     []asset.IngestFileMeta
	Streams   []smil.Video
	Subtitles []smil.TextStream
}

func MuxFiles(ctx workflow.Context, params MuxFilesParams) (*MuxFilesResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting MuxFiles")

	options := GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	ctx = workflow.WithTaskQueue(ctx, utils.GetTranscodeQueue())

	var files []asset.IngestFileMeta
	audioLanguages := utils.LanguageKeysToOrderedLanguages(lo.Keys(params.AudioFiles))
	if params.WithFiles {
		for _, lang := range audioLanguages {
			for _, q := range []string{r1080p, r540p, r180p} {
				base := filepath.Base(params.VideoFiles[q])
				key := lang.ISO6391
				fileName := base[:len(base)-len(filepath.Ext(base))] + "-" + key
				var result common.MuxResult
				err := workflow.ExecuteActivity(ctx, activities.TranscodeMux, common.MuxInput{
					FileName:          fileName,
					DestinationPath:   params.OutputPath,
					VideoFilePath:     params.VideoFiles[q],
					AudioFilePaths:    map[string]string{key: params.AudioFiles[key]},
					SubtitleFilePaths: params.SubtitleFiles,
				}).Get(ctx, &result)
				if err != nil {
					return nil, err
				}
				code := lang.ISO6392TwoLetter
				if code == "" {
					code = lang.ISO6391
				}
				files = append(files, asset.IngestFileMeta{
					Resolution:    q,
					AudioLanguage: code,
					Mime:          "video/mp4",
					Path:          filepath.Base(result.Path),
				})
			}
		}
	}

	var subtitles []smil.TextStream
	subtitleLanguages := utils.LanguageKeysToOrderedLanguages(lo.Keys(params.SubtitleFiles))
	for _, language := range subtitleLanguages {
		path := params.SubtitleFiles[language.ISO6391]
		subtitles = append(subtitles, smil.TextStream{
			Src:            filepath.Base(path),
			SystemLanguage: language.ISO6391,
			SubtitleName:   language.LanguageNameSystem,
		})
	}

	var streams []smil.Video
	languages := audioLanguages

	for _, q := range []string{r180p, r270p, r360p, r540p, r720p, r1080p} {
		path := params.VideoFiles[q]

		audioFilePaths := map[string]string{}
		var fileLanguages []bccmflows.Language
		// Add audio files to mux, but uniquely across qualities.
		for len(languages) > 0 && len(audioFilePaths) < 16 {
			key := languages[0].ISO6391
			fileLanguages = append(fileLanguages, languages[0])
			audioFilePaths[key] = params.AudioFiles[key]
			languages = languages[1:]
		}

		base := filepath.Base(path)
		fileName := base[:len(base)-len(filepath.Ext(base))]

		var result common.MuxResult
		err := workflow.ExecuteActivity(ctx, activities.TranscodeMux, common.MuxInput{
			FileName:        fileName,
			DestinationPath: params.OutputPath,
			AudioFilePaths:  audioFilePaths,
			VideoFilePath:   path,
		}).Get(ctx, &result)
		if err != nil {
			return nil, err
		}

		streams = append(streams, smil.Video{
			Src:          filepath.Base(result.Path),
			IncludeAudio: fmt.Sprintf("%t", len(fileLanguages) > 0),
			SystemLanguage: strings.Join(lo.Map(fileLanguages, func(i bccmflows.Language, _ int) string {
				return i.ISO6391
			}), ","),
			AudioName: strings.Join(lo.Map(fileLanguages, func(i bccmflows.Language, _ int) string {
				return i.LanguageNameSystem
			}), ","),
		})
	}

	return &MuxFilesResult{
		Files:     files,
		Streams:   streams,
		Subtitles: subtitles,
	}, nil
}
