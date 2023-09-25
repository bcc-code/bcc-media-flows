package transcode

import (
	"fmt"
	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/samber/lo"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type languageFile struct {
	Path     string
	Language string
}

// Order and respect the global language ordering.
func getMuxAudioFiles(input common.MuxInput) []languageFile {
	languages := utils.LanguageKeysToOrderedLanguages(lo.Keys(input.AudioFilePaths))

	return lo.Map(languages, func(lang bccmflows.Language, _ int) languageFile {
		return languageFile{
			Path:     input.AudioFilePaths[lang.ISO6391],
			Language: lang.ISO6391,
		}
	})
}

// Order and respect the global language ordering.
func getMuxSubtitleFiles(input common.MuxInput) []languageFile {
	languages := utils.LanguageKeysToOrderedLanguages(lo.Keys(input.SubtitleFilePaths))

	return lo.Map(languages, func(lang bccmflows.Language, _ int) languageFile {
		return languageFile{
			Path:     input.SubtitleFilePaths[lang.ISO6391],
			Language: lang.ISO6391,
		}
	})
}

// Mux multiplexes specified video, audio and subtitle tracks.
func Mux(input common.MuxInput, progressCallback ffmpeg.ProgressCallback) (*common.MuxResult, error) {
	//Use ffmpeg to mux the video
	info, err := ffmpeg.GetStreamInfo(input.VideoFilePath)
	if err != nil {
		return nil, err
	}

	outputPath := filepath.Join(input.DestinationPath, input.FileName+".mp4")

	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", input.VideoFilePath,
	}

	audioFiles := getMuxAudioFiles(input)
	subtitleFiles := getMuxSubtitleFiles(input)

	for _, f := range audioFiles {
		params = append(params,
			"-i", f.Path,
		)
	}

	for _, f := range subtitleFiles {
		params = append(params,
			"-i", f.Path,
		)
	}

	streams := 0
	params = append(
		params,
		"-map", fmt.Sprintf("%d:v", streams),
		fmt.Sprintf("-metadata:s:%d", streams), "language=eng",
	)
	streams++

	for _, f := range audioFiles {
		params = append(params,
			"-map", fmt.Sprintf("%d:a", streams),
			fmt.Sprintf("-metadata:s:%d", streams), fmt.Sprintf("language=%s", f.Language),
		)
		streams++
	}

	for _, f := range subtitleFiles {
		params = append(params,
			"-map", fmt.Sprintf("%d:s", streams),
			fmt.Sprintf("-metadata:s:%d", streams), fmt.Sprintf("language=%s", f.Language),
		)
		streams++
	}

	// using "copy" codec to avoid re-encoding, mov_text is the subtitle codec for mp4
	params = append(params,
		"-c:v", "copy",
		"-c:a", "copy",
		"-c:s", "mov_text",
		"-y", outputPath,
	)

	_, err = ffmpeg.Do(params, info, progressCallback)
	if err != nil {
		log.Default().Println("mux failed", err)
		return nil, fmt.Errorf("mux failed, %s", strings.Join(params, " "))
	}

	err = os.Chmod(outputPath, os.ModePerm)
	if err != nil {
		return nil, err
	}
	return &common.MuxResult{
		Path: outputPath,
	}, nil
}
