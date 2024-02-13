package export

import (
	"path/filepath"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/utils/workflows"

	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/common/smil"
	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/bcc-code/bcc-media-platform/backend/asset"
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

func getSubtitlesResult(subtitleFiles map[string]paths.Path) []smil.TextStream {
	var subtitles []smil.TextStream
	subtitleLanguages := utils.LanguageKeysToOrderedLanguages(lo.Keys(subtitleFiles))
	for _, language := range subtitleLanguages {
		path := subtitleFiles[language.ISO6391]
		subtitles = append(subtitles, smil.TextStream{
			Src:            path.Base(),
			SystemLanguage: language.ISO6391,
			SubtitleName:   language.LanguageNameSystem,
		})
	}
	return subtitles
}

var fileQualities = []quality{r1080p, r540p, r180p}

func createTranslatedFile(ctx workflow.Context, language string, videoPath, outputPath, audioPath paths.Path, subtitlePaths map[string]paths.Path) workflow.Future {
	base := videoPath.Base()
	fileName := base[:len(base)-len(filepath.Ext(base))] + "-" + language
	return wfutils.Execute(ctx, activities.TranscodeMux, common.MuxInput{
		FileName:          fileName,
		DestinationPath:   outputPath,
		VideoFilePath:     videoPath,
		AudioFilePaths:    map[string]paths.Path{language: audioPath},
		SubtitleFilePaths: subtitlePaths,
	})
}

var streamQualities = []quality{r180p, r270p, r360p, r540p, r720p, r1080p}

func getQualitiesWithLanguages(audioKeys []string) map[quality][]bccmflows.Language {
	languages := utils.LanguageKeysToOrderedLanguages(audioKeys)

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

func createStreamFile(ctx workflow.Context, q quality, videoFile, outputPath paths.Path, languageMapping map[quality][]bccmflows.Language, audioFiles map[string]paths.Path) workflow.Future {
	audioFilePaths := map[string]paths.Path{}
	for _, lang := range languageMapping[q] {
		audioFilePaths[lang.ISO6391] = audioFiles[lang.ISO6391]
	}

	base := videoFile.Base()
	fileName := base[:len(base)-len(filepath.Ext(base))]

	return wfutils.Execute(ctx, activities.TranscodeMux, common.MuxInput{
		FileName:        fileName,
		DestinationPath: outputPath,
		AudioFilePaths:  audioFilePaths,
		VideoFilePath:   videoFile,
	})
}
