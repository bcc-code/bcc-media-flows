package transcode

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/paths"
	"log"
	"os"
	"path/filepath"
	"strings"

	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/samber/lo"
)

type languageFile struct {
	Path     paths.Path
	Language string
}

// Order and respect the global language ordering.
func languageFilesForPaths(paths map[string]paths.Path) []languageFile {
	languages := utils.LanguageKeysToOrderedLanguages(lo.Keys(paths))

	return lo.Map(languages, func(lang bccmflows.Language, _ int) languageFile {
		return languageFile{
			Path:     paths[lang.ISO6391],
			Language: lang.ISO6391,
		}
	})
}

// Mux multiplexes specified video, audio and subtitle tracks.
func Mux(input common.MuxInput, progressCallback ffmpeg.ProgressCallback) (*common.MuxResult, error) {
	//Use ffmpeg to mux the video
	info, err := ffmpeg.GetStreamInfo(input.VideoFilePath.LocalPath())
	if err != nil {
		return nil, err
	}

	outputFilePath := filepath.Join(input.DestinationPath.LocalPath(), input.FileName+".mp4")

	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", input.VideoFilePath.LocalPath(),
	}

	audioFiles := languageFilesForPaths(input.AudioFilePaths)
	subtitleFiles := languageFilesForPaths(input.SubtitleFilePaths)

	for _, f := range audioFiles {
		params = append(params,
			"-i", f.Path.LocalPath(),
		)
	}

	for _, f := range subtitleFiles {
		params = append(params,
			"-i", f.Path.LocalPath(),
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
		"-y", outputFilePath,
	)

	_, err = ffmpeg.Do(params, info, progressCallback)
	if err != nil {
		log.Default().Println("mux failed", err)
		return nil, fmt.Errorf("mux failed, %s", strings.Join(params, " "))
	}

	err = os.Chmod(outputFilePath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	outputPath, err := paths.ParsePath(outputFilePath)
	if err != nil {
		return nil, err
	}

	return &common.MuxResult{
		Path: outputPath,
	}, nil
}
