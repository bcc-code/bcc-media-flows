package export

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/environment"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bcc-media-platform/backend/asset"
	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/common/smil"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
)

const (
	r1080p = "1920x1080"
	r720p  = "1280x720"
	r540p  = "960x540"
	r360p  = "640x360"
	r270p  = "480x270"
	r180p  = "320x180"
)

type MuxFilesParams struct {
	VideoFiles    map[string]paths.Path
	AudioFiles    map[string]paths.Path
	SubtitleFiles map[string]paths.Path
	OutputPath    paths.Path
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

	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)
	ctx = workflow.WithTaskQueue(ctx, environment.GetTranscodeQueue())

	languagesPerQuality := getLanguagesPerQuality(params)
	streamTasks := startStreamTasks(ctx, params, languagesPerQuality)

	audioLanguages := utils.LanguageKeysToOrderedLanguages(lo.Keys(params.AudioFiles))
	fileTasks := startFileTasks(ctx, params, audioLanguages)

	streams, err := waitForStreamTasks(ctx, streamTasks, languagesPerQuality)
	if err != nil {
		return nil, err
	}

	files, err := waitForFileTasks(ctx, params, fileTasks, audioLanguages)
	if err != nil {
		return nil, err
	}

	return &MuxFilesResult{
		Files:     files,
		Streams:   streams,
		Subtitles: getSubtitlesResult(params),
	}, nil
}

func getSubtitlesResult(params MuxFilesParams) []smil.TextStream {
	var subtitles []smil.TextStream
	subtitleLanguages := utils.LanguageKeysToOrderedLanguages(lo.Keys(params.SubtitleFiles))
	for _, language := range subtitleLanguages {
		path := params.SubtitleFiles[language.ISO6391]
		subtitles = append(subtitles, smil.TextStream{
			Src:            path.Base(),
			SystemLanguage: language.ISO6391,
			SubtitleName:   language.LanguageNameSystem,
		})
	}
	return subtitles
}

var fileQualities = []string{r1080p, r540p, r180p}

func startFileTasks(ctx workflow.Context, params MuxFilesParams, languages []bccmflows.Language) map[string]map[string]workflow.Future {
	var fileTasks = map[string]map[string]workflow.Future{}
	if params.WithFiles {
		for _, lang := range languages {
			key := lang.ISO6391
			fileTasks[key] = map[string]workflow.Future{}
			for _, q := range fileQualities {
				base := params.VideoFiles[q].Base()
				fileName := base[:len(base)-len(filepath.Ext(base))] + "-" + key
				fileTasks[key][q] = wfutils.ExecuteWithQueue(ctx, activities.TranscodeMux, common.MuxInput{
					FileName:          fileName,
					DestinationPath:   params.OutputPath,
					VideoFilePath:     params.VideoFiles[q],
					AudioFilePaths:    map[string]paths.Path{key: params.AudioFiles[key]},
					SubtitleFilePaths: params.SubtitleFiles,
				})
			}
		}
	}
	return fileTasks
}

func waitForFileTasks(ctx workflow.Context, params MuxFilesParams, tasks map[string]map[string]workflow.Future, languages []bccmflows.Language) ([]asset.IngestFileMeta, error) {
	var files []asset.IngestFileMeta
	if params.WithFiles {
		for _, lang := range languages {
			key := lang.ISO6391
			for _, q := range fileQualities {
				task := tasks[key][q]
				var result common.MuxResult
				err := task.Get(ctx, &result)
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
					Path:          result.Path.Base(),
				})
			}
		}
	}
	return files, nil
}

var streamQualities = []string{r180p, r270p, r360p, r540p, r720p, r1080p}

func getLanguagesPerQuality(params MuxFilesParams) map[string][]bccmflows.Language {
	languages := utils.LanguageKeysToOrderedLanguages(lo.Keys(params.AudioFiles))

	languagesPerQuality := map[string][]bccmflows.Language{}
	for _, q := range streamQualities {
		languagesPerQuality[q] = []bccmflows.Language{}
		for len(languages) > 0 && len(languagesPerQuality[q]) < 16 {
			languagesPerQuality[q] = append(languagesPerQuality[q], languages[0])
			languages = languages[1:]
		}
	}
	return languagesPerQuality
}

func startStreamTasks(ctx workflow.Context, params MuxFilesParams, languages map[string][]bccmflows.Language) map[string]workflow.Future {
	tasks := map[string]workflow.Future{}
	for _, q := range streamQualities {
		path := params.VideoFiles[q]

		audioFilePaths := map[string]paths.Path{}
		for _, lang := range languages[q] {
			key := lang.ISO6391
			audioFilePaths[key] = params.AudioFiles[key]
		}

		base := path.Base()
		fileName := base[:len(base)-len(filepath.Ext(base))]

		tasks[q] = workflow.ExecuteActivity(ctx, activities.TranscodeMux, common.MuxInput{
			FileName:        fileName,
			DestinationPath: params.OutputPath,
			AudioFilePaths:  audioFilePaths,
			VideoFilePath:   path,
		})
	}
	return tasks
}

func waitForStreamTasks(ctx workflow.Context, tasks map[string]workflow.Future, languages map[string][]bccmflows.Language) ([]smil.Video, error) {
	var streams []smil.Video
	for _, q := range streamQualities {
		task := tasks[q]
		var result common.MuxResult
		err := task.Get(ctx, &result)
		if err != nil {
			return nil, err
		}

		fileLanguages := languages[q]

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
	}
	return streams, nil
}
