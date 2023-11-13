package export

import (
	"path/filepath"

	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/utils/wfutils"

	"github.com/bcc-code/bcc-media-platform/backend/asset"
	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/common/smil"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
)

type quality string

const (
	r1080p = quality("1920x1080")
	r720p  = quality("1280x720")
	r540p  = quality("960x540")
	r360p  = quality("640x360")
	r270p  = quality("480x270")
	r180p  = quality("320x180")
)

type MuxFilesParams struct {
	VideoFiles    map[quality]paths.Path
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

var fileQualities = []quality{r1080p, r540p, r180p}

func startFileTasks(ctx workflow.Context, params MuxFilesParams, languages []bccmflows.Language, selector workflow.Selector, callback func(f common.MuxResult, l string, q quality)) {
	for _, key := range languages {
		lang := key.ISO6391
		for _, key := range fileQualities {
			q := key
			task := createTranslatedFile(ctx, lang, params.VideoFiles[q], params.OutputPath, params.AudioFiles[lang], params.SubtitleFiles)
			selector.AddFuture(task, func(f workflow.Future) {
				var result common.MuxResult
				err := f.Get(ctx, &result)
				if err != nil {
					workflow.GetLogger(ctx).Error("Failed to get mux result", "error", err)
					return
				}
				callback(result, lang, q)
			})
		}
	}
}

func createTranslatedFile(ctx workflow.Context, language string, videoPath, outputPath, audioPath paths.Path, subtitlePaths map[string]paths.Path) workflow.Future {
	base := videoPath.Base()
	fileName := base[:len(base)-len(filepath.Ext(base))] + "-" + language
	return wfutils.ExecuteWithQueue(ctx, activities.TranscodeMux, common.MuxInput{
		FileName:          fileName,
		DestinationPath:   outputPath,
		VideoFilePath:     videoPath,
		AudioFilePaths:    map[string]paths.Path{language: audioPath},
		SubtitleFilePaths: subtitlePaths,
	})
}

var streamQualities = []quality{r180p, r270p, r360p, r540p, r720p, r1080p}

func getQualitiesWithLanguages(params MuxFilesParams) map[quality][]bccmflows.Language {
	languages := utils.LanguageKeysToOrderedLanguages(lo.Keys(params.AudioFiles))

	languagesPerQuality := map[quality][]bccmflows.Language{}
	for _, q := range streamQualities {
		languagesPerQuality[q] = []bccmflows.Language{}
		for len(languages) > 0 && len(languagesPerQuality[q]) < 16 {
			languagesPerQuality[q] = append(languagesPerQuality[q], languages[0])
			languages = languages[1:]
		}
	}
	return languagesPerQuality
}

func startStreamTasks(ctx workflow.Context, params MuxFilesParams, qualities map[quality][]bccmflows.Language, selector workflow.Selector, callback func(r common.MuxResult, q quality)) {
	for _, key := range streamQualities {
		q := key
		path := params.VideoFiles[q]

		audioFilePaths := map[string]paths.Path{}
		for _, lang := range qualities[q] {
			audioFilePaths[lang.ISO6391] = params.AudioFiles[lang.ISO6391]
		}

		base := path.Base()
		fileName := base[:len(base)-len(filepath.Ext(base))]

		selector.AddFuture(workflow.ExecuteActivity(ctx, activities.TranscodeMux, common.MuxInput{
			FileName:        fileName,
			DestinationPath: params.OutputPath,
			AudioFilePaths:  audioFilePaths,
			VideoFilePath:   path,
		}), func(f workflow.Future) {
			var result common.MuxResult
			err := f.Get(ctx, &result)
			if err != nil {
				workflow.GetLogger(ctx).Error("Failed to get mux result", "error", err)
				return
			}
			callback(result, q)
		})
	}
}
