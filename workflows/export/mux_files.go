package export

import (
	"fmt"
	"github.com/bcc-code/bcc-media-platform/backend/asset"
	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/common/smil"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/workflows"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
	"path/filepath"
	"strings"
)

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

	options := workflows.GetDefaultActivityOptions()
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
