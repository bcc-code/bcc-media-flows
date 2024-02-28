package export

import (
	"fmt"
	"path/filepath"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
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
	r1080p = "1920x1080"
	r720p  = "1280x720"
	r540p  = "960x540"
	r360p  = "640x360"
	r270p  = "480x270"
	r180p  = "320x180"
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

var fileQualities = []string{r1080p, r540p, r180p}

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

func getQualitiesWithLanguages(audioKeys []string, resolutions []vsapi.Resolution) map[string][]bccmflows.Language {
	languages := utils.LanguageKeysToOrderedLanguages(audioKeys)

	languagesPerQuality := map[string][]bccmflows.Language{}

	var sortedByHeightAsc []vsapi.Resolution
	for _, r := range resolutions {
		if len(sortedByHeightAsc) == 0 {
			sortedByHeightAsc = append(sortedByHeightAsc, r)
			continue
		}
		if sortedByHeightAsc[len(sortedByHeightAsc)-1].Height > r.Height {
			sortedByHeightAsc = append(sortedByHeightAsc, r)
		} else {
			for i, s := range sortedByHeightAsc {
				if s.Height > r.Height {
					sortedByHeightAsc = append(sortedByHeightAsc[:i], append([]vsapi.Resolution{r}, sortedByHeightAsc[i:]...)...)
					break
				}
			}
		}
	}

	for _, r := range sortedByHeightAsc {
		q := fmt.Sprintf("%dx%d", r.Width, r.Height)
		languagesPerQuality[q] = []bccmflows.Language{}
		for len(languages) > 0 && len(languagesPerQuality[q]) < 16 {
			languagesPerQuality[q] = append(languagesPerQuality[q], languages[0])
			languages = languages[1:]
		}
	}

	return languagesPerQuality
}

func createStreamFile(ctx workflow.Context, q string, videoFile, outputPath paths.Path, languageMapping map[string][]bccmflows.Language, audioFiles map[string]paths.Path) workflow.Future {
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
