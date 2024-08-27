package export

import (
	"path/filepath"
	"sort"

	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"

	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/common/smil"
	"github.com/bcc-code/bcc-media-flows/utils"
	"go.temporal.io/sdk/workflow"
)

type quality string

func getSubtitlesResult(ctx workflow.Context, subtitleFiles map[string]paths.Path) []smil.TextStream {
	var subtitles []smil.TextStream
	keys, _ := wfutils.GetMapKeysSafely(ctx, subtitleFiles)
	subtitleLanguages := utils.LanguageKeysToOrderedLanguages(keys)
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

func createTranslatedFile(ctx workflow.Context, language string, videoPath, outputPath, audioPath paths.Path, subtitlePaths map[string]paths.Path) workflow.Future {
	base := videoPath.Base()
	fileName := base[:len(base)-len(filepath.Ext(base))] + "-" + language
	return wfutils.Execute(ctx, activities.Audio.TranscodeMux, common.MuxInput{
		FileName:          fileName,
		DestinationPath:   outputPath,
		VideoFilePath:     videoPath,
		AudioFilePaths:    map[string]paths.Path{language: audioPath},
		SubtitleFilePaths: subtitlePaths,
	}).Future
}

type ResolutionWithLanguages struct {
	Resolution resolutionString
	Languages  []bccmflows.Language
}

func assignLanguagesToResolutions(audioKeys []string, resolutions []utils.Resolution) []ResolutionWithLanguages {
	languages := utils.LanguageKeysToOrderedLanguages(audioKeys)

	sortedResolutions := sortResolutionsForVODStreaming(resolutions)

	qualities := make([]ResolutionWithLanguages, len(sortedResolutions))
	for i, r := range sortedResolutions {
		qualities[i] = ResolutionWithLanguages{
			Resolution: resolutionToString(r),
			Languages:  []bccmflows.Language{},
		}
		for len(languages) > 0 && len(qualities[i].Languages) < 16 {
			qualities[i].Languages = append(qualities[i].Languages, languages[0])
			languages = languages[1:]
		}
	}

	return qualities
}

// sortResolutionsForVODStreaming sorts resolutions so that 540p is first, then ascending height
func sortResolutionsForVODStreaming(resolutions []utils.Resolution) []utils.Resolution {
	sortedResolutions := make([]utils.Resolution, len(resolutions))
	copy(sortedResolutions, resolutions)

	sort.Slice(sortedResolutions, func(i, j int) bool {
		if sortedResolutions[i].Height == 540 {
			return true
		}
		if sortedResolutions[j].Height == 540 {
			return false
		}
		return sortedResolutions[i].Height < sortedResolutions[j].Height
	})

	return sortedResolutions
}

func createStreamFile(ctx workflow.Context, languages []bccmflows.Language, videoFile, outputPath paths.Path, audioFiles map[string]paths.Path) workflow.Future {
	audioFilePaths := map[string]paths.Path{}
	for _, lang := range languages {
		audioFilePaths[lang.ISO6391] = audioFiles[lang.ISO6391]
	}

	base := videoFile.Base()
	fileName := base[:len(base)-len(filepath.Ext(base))]

	return wfutils.Execute(ctx, activities.Audio.TranscodeMux, common.MuxInput{
		FileName:        fileName,
		DestinationPath: outputPath,
		AudioFilePaths:  audioFilePaths,
		VideoFilePath:   videoFile,
	}).Future
}
